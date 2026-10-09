package pluginscripts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	pinned   = "0.2.0"
	repoPath = "/repos/brettcallaghan/release/releases"
)

// installCLI is the script under test, relative to this package.
var installCLI = filepath.Join("..", "..", "plugin", "skills", "cut-release", "scripts", "install-cli")

// The tools the script may use, linked into a directory that is the whole of
// its PATH, so nothing on the machine running the tests (a real `release`,
// a logged-in `gh`) can leak into a case.
var tools = []string{
	"bash", "sh", "curl", "tar", "gzip", "sha256sum", "shasum", "awk", "sed",
	"mkdir", "rm", "mv", "chmod", "ln", "cat", "dirname", "git", "uname",
	"tr", "env", "sleep",
}

// fakeAPI stands in for api.github.com and for the host its asset downloads
// redirect to. The asset host is reached as "localhost" and the API as
// "127.0.0.1", so the redirect crosses hosts as the real one does.
type fakeAPI struct {
	api, assets *httptest.Server

	mu         sync.Mutex
	files      map[string][]byte // asset name -> bytes
	ids        map[int]string    // asset id -> name
	tagStatus  int               // non-zero: the tags lookup answers with it
	apiAuth    []string          // Authorization of every API request
	assetAuth  []string          // Authorization of every asset-host request
	downloaded []string          // asset names, in download order
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{files: map[string][]byte{}, ids: map[int]string{}}
	f.assets = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.assetAuth = append(f.assetAuth, r.Header.Get("Authorization"))
		body, ok := f.files[strings.TrimPrefix(r.URL.Path, "/")]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(f.assets.Close)
	assetBase := strings.Replace(f.assets.URL, "127.0.0.1", "localhost", 1)

	f.api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.apiAuth = append(f.apiAuth, r.Header.Get("Authorization"))
		switch {
		case r.URL.Path == repoPath+"/tags/v"+pinned:
			if f.tagStatus != 0 {
				w.WriteHeader(f.tagStatus)
				_, _ = w.Write([]byte(`{"message":"Not Found"}`))
				return
			}
			_, _ = w.Write(f.releaseJSON())
		case strings.HasPrefix(r.URL.Path, repoPath+"/assets/"):
			var id int
			_, _ = fmt.Sscanf(strings.TrimPrefix(r.URL.Path, repoPath+"/assets/"), "%d", &id)
			name, ok := f.ids[id]
			if !ok || r.Header.Get("Accept") != "application/octet-stream" {
				http.NotFound(w, r)
				return
			}
			f.downloaded = append(f.downloaded, name)
			http.Redirect(w, r, assetBase+"/"+name, http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.api.Close)
	return f
}

// publish adds the assets of release v<pinned>: each named archive holding a
// binary that reports version, plus a SHA256SUMS covering them.
func (f *fakeAPI) publish(t *testing.T, version string, archives ...string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	var sums strings.Builder
	for _, name := range archives {
		data := archive(t, name, version)
		f.add(name, data)
		sum := sha256.Sum256(data)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	f.add("SHA256SUMS", []byte(sums.String()))
}

func (f *fakeAPI) add(name string, data []byte) {
	f.files[name] = data
	f.ids[1001+len(f.ids)] = name
}

// corrupt replaces an archive's bytes without changing SHA256SUMS.
func (f *fakeAPI) corrupt(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[name] = append([]byte("tampered"), f.files[name]...)
}

// releaseJSON is a release in the API's shape: nested objects with their own
// ids, and a body full of the characters a naive parser would trip on.
func (f *fakeAPI) releaseJSON() []byte {
	ids := make([]int, 0, len(f.ids))
	for id := range f.ids {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	assets := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		assets = append(assets, map[string]any{
			"url":      fmt.Sprintf("https://api.github.com%s/assets/%d", repoPath, id),
			"id":       id,
			"node_id":  "RA_x",
			"name":     f.ids[id],
			"label":    "",
			"uploader": map[string]any{"login": "bot", "id": 77, "name": "release_" + pinned + "_linux_amd64.tar.gz"},
			"size":     len(f.files[f.ids[id]]),
		})
	}
	out, _ := json.MarshalIndent(map[string]any{
		"id":       1,
		"tag_name": "v" + pinned,
		"name":     "v" + pinned,
		"author":   map[string]any{"login": "bot", "id": 77},
		"body":     `see {"id": 5, "name": "SHA256SUMS"} and "quoted \" text"`,
		"assets":   assets,
	}, "", "  ")
	return out
}

func (f *fakeAPI) requests() (api, asset []string, downloaded []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.apiAuth...), append([]string(nil), f.assetAuth...), append([]string(nil), f.downloaded...)
}

// archive builds a release archive named like scripts/build-release.sh's:
// the binary alone at its root.
func archive(t *testing.T, name, version string) []byte {
	t.Helper()
	exe, bin := "release", fakeRelease(version)
	var buf bytes.Buffer
	if strings.HasSuffix(name, ".zip") {
		exe = "release.exe"
		zw := zip.NewWriter(&buf)
		h := &zip.FileHeader{Name: exe, Method: zip.Deflate}
		h.SetMode(0o755)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(bin)
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: exe, Mode: 0o755, Size: int64(len(bin))}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write(bin)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeRelease is a stand-in binary: it answers --version as the CLI does.
func fakeRelease(version string) []byte {
	return []byte("#!/bin/sh\necho \"release v" + version + "\"\n")
}

// harness is one run's world: a plugin tree pinning a version, a data
// directory, a home and a working directory, and a PATH holding only the
// tools above plus whatever the case adds.
type harness struct {
	t                                *testing.T
	root, script, data, home, work   string
	extra, toolDir, linkDir, curlLog string
	stdin                            *os.File
	api                              *fakeAPI
	env                              map[string]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install-cli is exercised under bash on unix-like systems")
	}
	for _, need := range []string{"bash", "curl"} {
		if _, err := exec.LookPath(need); err != nil {
			t.Skipf("%s is not on the PATH", need)
		}
	}
	root := t.TempDir()
	h := &harness{
		t:       t,
		root:    root,
		data:    filepath.Join(root, "data"),
		home:    filepath.Join(root, "home"),
		work:    filepath.Join(root, "work"),
		extra:   filepath.Join(root, "extra"),
		toolDir: filepath.Join(root, "tools"),
		linkDir: filepath.Join(root, "link"),
		curlLog: filepath.Join(root, "curl-args.log"),
		api:     newFakeAPI(t),
		env:     map[string]string{},
	}
	for _, d := range []string{h.home, h.work, h.extra, h.toolDir, h.linkDir} {
		mustMkdir(t, d)
	}
	for _, tool := range tools {
		if p, err := exec.LookPath(tool); err == nil {
			if err := os.Symlink(p, filepath.Join(h.toolDir, tool)); err != nil {
				t.Fatal(err)
			}
		}
	}
	src, err := os.ReadFile(installCLI)
	if err != nil {
		t.Fatal(err)
	}
	h.script = filepath.Join(root, "plugin", "skills", "cut-release", "scripts", "install-cli")
	mustMkdir(t, filepath.Dir(h.script))
	mustWrite(t, h.script, src, 0o755)
	h.pin(pinned)
	h.platform("Linux", "x86_64")
	return h
}

// pin writes the plugin manifest the script reads its version from.
func (h *harness) pin(version string) {
	manifest := filepath.Join(h.root, "plugin", ".claude-plugin", "plugin.json")
	mustMkdir(h.t, filepath.Dir(manifest))
	mustWrite(h.t, manifest, []byte(`{
  "name": "release",
  "version": "`+version+`",
  "dependencies": ["specs@brettcallaghan"]
}
`), 0o644)
}

// platform makes uname report the given kernel and machine.
func (h *harness) platform(s, m string) {
	h.tool("uname", "#!/bin/sh\ncase \"$1\" in -s) echo '"+s+"' ;; -m) echo '"+m+"' ;; *) echo '"+s+"' ;; esac\n")
}

// tool puts an executable script on the case's PATH, ahead of the tools.
func (h *harness) tool(name, body string) string {
	p := filepath.Join(h.extra, name)
	mustWrite(h.t, p, []byte(body), 0o755)
	return p
}

// logCurl wraps curl so every argument list it is given is recorded.
func (h *harness) logCurl() {
	real, err := exec.LookPath("curl")
	if err != nil {
		h.t.Fatal(err)
	}
	h.tool("curl", "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+h.curlLog+"'\nexec '"+real+"' \"$@\"\n")
}

func (h *harness) run() (stdout, stderr string, code int) {
	h.t.Helper()
	cmd := exec.Command("bash", h.script, h.data)
	cmd.Dir = h.work
	cmd.Stdin = h.stdin
	cmd.Env = []string{
		"PATH=" + h.extra + string(os.PathListSeparator) + h.toolDir,
		"HOME=" + h.home,
		"TMPDIR=" + h.home,
		"INSTALL_CLI_API_URL=" + h.api.api.URL,
		"INSTALL_CLI_LINK_DIR=" + h.linkDir,
	}
	for k, v := range h.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		h.t.Fatalf("running install-cli: %v", err)
	}
	return out.String(), errOut.String(), code
}

// succeed runs the script, requires success, and returns the printed path.
func (h *harness) succeed() string {
	h.t.Helper()
	stdout, stderr, code := h.run()
	if code != 0 {
		h.t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr)
	}
	if stderr != "" {
		h.t.Errorf("stderr = %q, want empty", stderr)
	}
	if strings.Count(stdout, "\n") != 1 || !strings.HasSuffix(stdout, "\n") {
		h.t.Fatalf("stdout = %q, want exactly one line", stdout)
	}
	path := strings.TrimSuffix(stdout, "\n")
	if !filepath.IsAbs(path) {
		h.t.Errorf("printed path %q is not absolute", path)
	}
	return path
}

// fail runs the script, requires the one-line failure, and returns it.
func (h *harness) fail() string {
	h.t.Helper()
	stdout, stderr, code := h.run()
	if code != 1 {
		h.t.Fatalf("exit = %d, want 1; stdout: %q stderr: %q", code, stdout, stderr)
	}
	if stdout != "" {
		h.t.Errorf("stdout = %q, want empty on failure", stdout)
	}
	if strings.Count(stderr, "\n") != 1 || !strings.HasPrefix(stderr, "install-cli: ") {
		h.t.Fatalf("stderr = %q, want one line starting %q", stderr, "install-cli: ")
	}
	return strings.TrimSuffix(stderr, "\n")
}

// version runs a binary the way a skill would and returns what it reports.
func (h *harness) version(path string) string {
	h.t.Helper()
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		h.t.Fatalf("%s --version: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

func (h *harness) provisioned(exe string) string {
	return filepath.Join(h.data, "bin", pinned, exe)
}

// requireNoTemp checks that no download directory outlived the run.
func (h *harness) requireNoTemp() {
	h.t.Helper()
	entries, _ := os.ReadDir(filepath.Join(h.data, "bin"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			h.t.Errorf("temporary directory %s was left behind", e.Name())
		}
	}
}

func (h *harness) requireNoRequests() {
	h.t.Helper()
	api, asset, _ := h.api.requests()
	if len(api)+len(asset) != 0 {
		h.t.Errorf("made %d API and %d asset requests, want none", len(api), len(asset))
	}
}

// AC: AC-PLG-001.1
func TestPinnedBuildForThePlatformIsProvisioned(t *testing.T) {
	for _, tc := range []struct{ uname, machine, archive, exe string }{
		{"Linux", "x86_64", "release_0.2.0_linux_amd64.tar.gz", "release"},
		{"Linux", "aarch64", "release_0.2.0_linux_arm64.tar.gz", "release"},
		{"Darwin", "x86_64", "release_0.2.0_darwin_amd64.tar.gz", "release"},
		{"Darwin", "arm64", "release_0.2.0_darwin_arm64.tar.gz", "release"},
		{"MINGW64_NT-10.0", "x86_64", "release_0.2.0_windows_amd64.zip", "release.exe"},
		{"MSYS_NT-10.0", "aarch64", "release_0.2.0_windows_arm64.zip", "release.exe"},
	} {
		t.Run(tc.archive, func(t *testing.T) {
			h := newHarness(t)
			if strings.HasSuffix(tc.archive, ".zip") {
				requireZipTar(t, h)
			}
			h.platform(tc.uname, tc.machine)
			h.api.publish(t, pinned,
				"release_0.2.0_linux_amd64.tar.gz", "release_0.2.0_linux_arm64.tar.gz",
				"release_0.2.0_darwin_amd64.tar.gz", "release_0.2.0_darwin_arm64.tar.gz",
				"release_0.2.0_windows_amd64.zip", "release_0.2.0_windows_arm64.zip")

			path := h.succeed()
			if path != h.provisioned(tc.exe) {
				t.Errorf("path = %s, want %s", path, h.provisioned(tc.exe))
			}
			if got := h.version(path); got != "release v"+pinned {
				t.Errorf("--version = %q, want %q", got, "release v"+pinned)
			}
			_, _, downloaded := h.api.requests()
			if len(downloaded) != 2 || downloaded[0] != "SHA256SUMS" || downloaded[1] != tc.archive {
				t.Errorf("downloaded %v, want [SHA256SUMS %s]", downloaded, tc.archive)
			}
			h.requireNoTemp()
		})
	}
}

// requireZipTar skips a windows row where tar cannot read a zip archive.
// GNU tar cannot; bsdtar (macOS, Windows) can.
func requireZipTar(t *testing.T, h *harness) {
	t.Helper()
	z := filepath.Join(h.root, "probe.zip")
	mustWrite(t, z, archive(t, "probe.zip", pinned), 0o644)
	if err := exec.Command("tar", "-tf", z).Run(); err != nil {
		t.Skip("tar on this machine cannot read zip archives; the row runs where tar is bsdtar")
	}
}

// AC: AC-PLG-001.2
func TestProvisionedPinnedVersionIsReusedWithoutTheNetwork(t *testing.T) {
	h := newHarness(t)
	mustMkdir(t, filepath.Dir(h.provisioned("release")))
	mustWrite(t, h.provisioned("release"), fakeRelease(pinned), 0o755)
	h.api.api.Close() // the network is unavailable

	path := h.succeed()
	if path != h.provisioned("release") {
		t.Errorf("path = %s, want %s", path, h.provisioned("release"))
	}
	if got := h.version(path); got != "release v"+pinned {
		t.Errorf("--version = %q", got)
	}
}

// AC: AC-PLG-001.3
func TestPluginUpdateProvisionsItsOwnVersion(t *testing.T) {
	h := newHarness(t)
	old := filepath.Join(h.data, "bin", "0.1.0", "release")
	mustMkdir(t, filepath.Dir(old))
	mustWrite(t, old, fakeRelease("0.1.0"), 0o755)
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

	path := h.succeed()
	if got := h.version(path); got != "release v0.2.0" {
		t.Errorf("--version = %q, want release v0.2.0", got)
	}
	if got := h.version(old); got != "release v0.1.0" {
		t.Errorf("the 0.1.0 install was disturbed: %q", got)
	}
}

// AC: AC-PLG-001.5
func TestArchiveFailingItsChecksumIsNeverExtracted(t *testing.T) {
	h := newHarness(t)
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")
	h.api.corrupt("release_0.2.0_linux_amd64.tar.gz")

	msg := h.fail()
	want := "install-cli: checksum mismatch for release_0.2.0_linux_amd64.tar.gz. retry; a repeated mismatch is a corrupted release to report"
	if msg != want {
		t.Errorf("stderr = %q\nwant      %q", msg, want)
	}
	if _, err := os.Stat(filepath.Join(h.data, "bin", pinned)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("bin/%s exists after a checksum mismatch (err %v)", pinned, err)
	}
	h.requireNoTemp()
	walk(t, h.data, func(rel string, _ fs.FileInfo) {
		if strings.HasSuffix(rel, ".tar.gz") || filepath.Base(rel) == "release" {
			t.Errorf("%s was kept after a checksum mismatch", rel)
		}
	})
}

// AC: AC-PLG-001.6
func TestProvisioningFailureIsOneMessageNamingCauseAndRemedy(t *testing.T) {
	for _, tc := range []struct {
		cause, want string
		arrange     func(*harness)
	}{
		{
			"the platform has no release archive",
			"install-cli: no release archive for SunOS/sun4v. use linux, darwin or windows on amd64 or arm64",
			func(h *harness) { h.platform("SunOS", "sun4v") },
		},
		{
			"the release cannot be reached",
			"install-cli: the release v0.2.0 cannot be reached (the release). check that the session can reach api.github.com",
			func(h *harness) { h.api.api.Close() },
		},
		{
			"no credential can read the repository",
			"install-cli: the release v0.2.0 is not readable (HTTP 404 for the release). set GH_TOKEN to a token that can read brettcallaghan/release, or run gh auth login",
			func(h *harness) { h.api.tagStatus = http.StatusNotFound },
		},
		{
			"the archive fails its checksum",
			"install-cli: checksum mismatch for release_0.2.0_linux_amd64.tar.gz. retry; a repeated mismatch is a corrupted release to report",
			func(h *harness) {
				h.api.publish(h.t, pinned, "release_0.2.0_linux_amd64.tar.gz")
				h.api.corrupt("release_0.2.0_linux_amd64.tar.gz")
			},
		},
	} {
		t.Run(tc.cause, func(t *testing.T) {
			h := newHarness(t)
			tc.arrange(h)
			if msg := h.fail(); msg != tc.want {
				t.Errorf("stderr = %q\nwant      %q", msg, tc.want)
			}
			h.requireNoTemp()
		})
	}
}

// AC: AC-PLG-002.1
func TestCloudSessionProvisionsIntoTheDataDirAndLinksRelease(t *testing.T) {
	h := newHarness(t)
	h.env["CLAUDE_CODE_REMOTE"] = "true"
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")
	// A stale link from an earlier version is replaced.
	if err := os.Symlink(filepath.Join(h.data, "bin", "0.1.0", "release"), filepath.Join(h.linkDir, "release")); err != nil {
		t.Fatal(err)
	}

	path := h.succeed()
	if path != h.provisioned("release") {
		t.Errorf("path = %s, want it under the data directory", path)
	}
	link := filepath.Join(h.linkDir, "release")
	if target, err := os.Readlink(link); err != nil || target != path {
		t.Errorf("link %s -> %q (err %v), want -> %s", link, target, err, path)
	}
	if got := h.version(link); got != "release v"+pinned {
		t.Errorf("release via the link reports %q", got)
	}
}

// AC: AC-PLG-002.2
func TestLocalSessionChangesNothingOutsideTheDataDir(t *testing.T) {
	h := newHarness(t)
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")
	before := snapshot(t, h.root, h.data)

	path := h.succeed()
	if !strings.HasPrefix(path, h.data+string(filepath.Separator)) {
		t.Errorf("path %s is outside the data directory", path)
	}
	after := snapshot(t, h.root, h.data)
	for p, v := range after {
		if before[p] != v {
			t.Errorf("%s was created or changed outside the data directory", p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			t.Errorf("%s was removed outside the data directory", p)
		}
	}
}

// AC: AC-PLG-002.3
func TestReleaseOnPathAtThePinnedVersionIsUsedAsFound(t *testing.T) {
	h := newHarness(t)
	onPath := h.tool("release", string(fakeRelease(pinned)))

	if path := h.succeed(); path != onPath {
		t.Errorf("path = %s, want the release on the PATH, %s", path, onPath)
	}
	h.requireNoRequests()
	if _, err := os.Stat(h.data); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the data directory was touched (err %v)", err)
	}
}

// A release on the PATH that never returns is given up on, and the probe
// reads end-of-file on stdin.
func TestReleaseOnPathThatBlocksFallsThroughToTheDownload(t *testing.T) {
	h := newHarness(t)
	h.env["INSTALL_CLI_PROBE_TIMEOUT"] = "1"
	onPath := h.tool("release", "#!/bin/sh\nexec sleep 1000\n")
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

	start := time.Now()
	path := h.succeed()
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("took %v, want the probe abandoned after about 1s", elapsed)
	}
	if path == onPath || path != h.provisioned("release") {
		t.Errorf("path = %s, want the downloaded release", path)
	}
}

// pidLog wraps sleep so each one started puts its PID in a file, and returns
// the file; alive reads it back.
func (h *harness) pidLog() string {
	h.t.Helper()
	real, err := exec.LookPath("sleep")
	if err != nil {
		h.t.Fatal(err)
	}
	log := filepath.Join(h.root, "sleep-pids")
	h.tool("sleep", "#!/bin/sh\necho $$ >> '"+log+"'\nexec '"+real+"' \"$@\"\n")
	return log
}

func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

func requireNoneAlive(t *testing.T, pidFile string) {
	t.Helper()
	data, _ := os.ReadFile(pidFile)
	for _, f := range strings.Fields(string(data)) {
		pid, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("bad pid %q", f)
		}
		deadline := time.Now().Add(2 * time.Second)
		for alive(pid) && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if alive(pid) {
			t.Errorf("process %d outlived install-cli", pid)
			p, _ := os.FindProcess(pid)
			_ = p.Kill()
		}
	}
}

// A release that ignores TERM and later prints the pinned version is not
// accepted, and nothing the probe started outlives the run.
func TestReleaseOnPathThatIgnoresTermIsNotAccepted(t *testing.T) {
	h := newHarness(t)
	h.env["INSTALL_CLI_PROBE_TIMEOUT"] = "1"
	pids := filepath.Join(h.root, "child-pids")
	onPath := h.tool("release", "#!/bin/bash\ntrap '' TERM\nsleep 4 &\necho $! >> '"+pids+"'\nwait\necho 'release v"+pinned+"'\n")
	sleeps := h.pidLog()
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

	start := time.Now()
	path := h.succeed()
	if elapsed := time.Since(start); elapsed > 3*time.Second+time.Second {
		t.Errorf("took %v, want the probe abandoned after about 1s", elapsed)
	}
	if path == onPath || path != h.provisioned("release") {
		t.Errorf("path = %s, want the downloaded release", path)
	}
	requireNoneAlive(t, pids)
	requireNoneAlive(t, sleeps)
}

// A probe that answers at once leaves no sleep behind, whatever the limit.
func TestReleaseOnPathProbeLeavesNoSleepBehind(t *testing.T) {
	h := newHarness(t)
	h.env["INSTALL_CLI_PROBE_TIMEOUT"] = "77"
	onPath := h.tool("release", string(fakeRelease(pinned)))
	sleeps := h.pidLog()
	if path := h.succeed(); path != onPath {
		t.Errorf("path = %s, want the release on the PATH, %s", path, onPath)
	}
	requireNoneAlive(t, sleeps)
}

// The probe needs no temporary file: an unwritable TMPDIR changes nothing.
func TestReleaseOnPathIsUsedWhenTmpdirIsUnwritable(t *testing.T) {
	h := newHarness(t)
	h.env["TMPDIR"] = filepath.Join(h.root, "does-not-exist")
	onPath := h.tool("release", string(fakeRelease(pinned)))
	if path := h.succeed(); path != onPath {
		t.Errorf("path = %s, want the release on the PATH, %s", path, onPath)
	}
	h.requireNoRequests()
}

func TestReleaseOnPathProbeReadsEndOfFileOnStdin(t *testing.T) {
	h := newHarness(t)
	h.env["INSTALL_CLI_PROBE_TIMEOUT"] = "30"
	// Script stdin is a pipe nobody writes to or closes: without /dev/null on
	// the probe, `read` blocks on it.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	h.stdin = r
	onPath := h.tool("release", "#!/bin/sh\nif read -r _line; then exit 1; fi\necho 'release v"+pinned+"'\n")
	if path := h.succeed(); path != onPath {
		t.Errorf("path = %s, want the release on the PATH, %s", path, onPath)
	}
}

// AC: AC-PLG-002.5
func TestReleaseOnPathAtAnotherVersionIsNeverUsed(t *testing.T) {
	h := newHarness(t)
	old := string(fakeRelease("0.1.0"))
	onPath := h.tool("release", old)
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

	path := h.succeed()
	if path == onPath {
		t.Fatal("the release on the PATH at 0.1.0 was chosen")
	}
	if got := h.version(path); got != "release v"+pinned {
		t.Errorf("--version = %q, want the pinned version", got)
	}
	if got, _ := os.ReadFile(onPath); string(got) != old {
		t.Error("the release on the PATH was changed")
	}
}

// AC: AC-PLG-002.4
func TestRepositoryCanDirectProvisioningToItsOwnBuild(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the PATH")
	}
	h := newHarness(t)
	if out, err := exec.Command("git", "init", "-q", h.work).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	own := filepath.Join(h.work, "build", "release")
	mustMkdir(t, filepath.Dir(own))
	mustWrite(t, own, fakeRelease("9.9.9"), 0o755)
	sub := filepath.Join(h.work, "nested", "dir")
	mustMkdir(t, sub)
	h.work = sub // relative paths resolve against the toplevel, not the cwd
	h.env["RELEASE_CLI"] = "build/release"
	h.env["CLAUDE_CODE_REMOTE"] = "true"

	// git reports the toplevel with symlinks resolved (macOS's /var is
	// /private/var), so compare resolved paths.
	want, err := filepath.EvalSymlinks(own)
	if err != nil {
		t.Fatal(err)
	}
	if path := h.succeed(); path != want {
		t.Errorf("path = %s, want %s", path, want)
	}
	h.requireNoRequests()
	if _, err := os.Stat(h.data); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the data directory was touched (err %v)", err)
	}
	if entries, _ := os.ReadDir(h.linkDir); len(entries) != 0 {
		t.Errorf("something was linked for the repository's own build: %v", entries)
	}
}

// AC: AC-PLG-002.4
func TestReleaseCLIThatIsNotExecutableFailsWithoutFallingThrough(t *testing.T) {
	h := newHarness(t)
	mustMkdir(t, filepath.Dir(h.provisioned("release")))
	mustWrite(t, h.provisioned("release"), fakeRelease(pinned), 0o755)
	notExec := filepath.Join(h.work, "release")
	mustWrite(t, notExec, fakeRelease(pinned), 0o644)
	h.env["RELEASE_CLI"] = notExec

	msg := h.fail()
	want := "install-cli: RELEASE_CLI is not an executable: " + notExec + ". fix or unset RELEASE_CLI"
	if msg != want {
		t.Errorf("stderr = %q\nwant      %q", msg, want)
	}
	h.requireNoRequests()
}

// AC: AC-PLG-003.1
func TestFirstAvailableCredentialIsUsed(t *testing.T) {
	gh := "#!/bin/sh\n[ \"$1 $2\" = 'auth token' ] && { echo gh-login-token; exit 0; }\nexit 1\n"
	for _, tc := range []struct {
		source string
		env    map[string]string
		gh     string
		want   string
	}{
		{"GH_TOKEN", map[string]string{"GH_TOKEN": "gh-token-env", "GITHUB_TOKEN": "github-token-env"}, gh, "Bearer gh-token-env"},
		{"GITHUB_TOKEN", map[string]string{"GITHUB_TOKEN": "github-token-env"}, gh, "Bearer github-token-env"},
		{"the GitHub CLI's stored login", nil, gh, "Bearer gh-login-token"},
		{"the session's own repository access", nil, "#!/bin/sh\necho 'not logged in' >&2\nexit 1\n", ""},
	} {
		t.Run(tc.source, func(t *testing.T) {
			h := newHarness(t)
			for k, v := range tc.env {
				h.env[k] = v
			}
			h.tool("gh", tc.gh)
			h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

			if got := h.version(h.succeed()); got != "release v"+pinned {
				t.Errorf("--version = %q", got)
			}
			api, _, _ := h.api.requests()
			if len(api) != 3 {
				t.Fatalf("made %d API requests, want 3", len(api))
			}
			for i, auth := range api {
				if auth != tc.want {
					t.Errorf("API request %d sent Authorization %q, want %q", i, auth, tc.want)
				}
			}
		})
	}
}

// AC: AC-PLG-003.4
func TestPublicReleaseNeedsNoCredential(t *testing.T) {
	h := newHarness(t) // no token in the environment and no gh on the PATH
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

	if got := h.version(h.succeed()); got != "release v"+pinned {
		t.Errorf("--version = %q", got)
	}
	api, asset, _ := h.api.requests()
	for _, auth := range append(api, asset...) {
		if auth != "" {
			t.Errorf("a request carried Authorization %q", auth)
		}
	}
}

const secret = "ghp_secretTokenNeverRecorded"

// AC: AC-PLG-003.2
func TestCredentialIsNeverRecorded(t *testing.T) {
	for _, outcome := range []string{"success", "failure"} {
		t.Run(outcome, func(t *testing.T) {
			h := newHarness(t)
			h.env["GH_TOKEN"] = secret
			h.env["CLAUDE_CODE_REMOTE"] = "true"
			h.logCurl()
			h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")
			if outcome == "failure" {
				h.api.corrupt("release_0.2.0_linux_amd64.tar.gz")
			}

			stdout, stderr, _ := h.run()
			if strings.Contains(stdout+stderr, secret) {
				t.Error("the credential appears in the script's output")
			}
			args, err := os.ReadFile(h.curlLog)
			if err != nil || len(args) == 0 {
				t.Fatalf("curl was not run through the wrapper: %v", err)
			}
			if strings.Contains(string(args), secret) {
				t.Errorf("the credential was passed on curl's command line:\n%s", args)
			}
			for _, dir := range []string{h.data, h.home, h.work, h.linkDir} {
				walk(t, dir, func(rel string, info fs.FileInfo) {
					if !info.Mode().IsRegular() {
						return
					}
					if b, _ := os.ReadFile(filepath.Join(dir, rel)); bytes.Contains(b, []byte(secret)) {
						t.Errorf("the credential was written to %s", filepath.Join(dir, rel))
					}
				})
			}
			if api, _, _ := h.api.requests(); len(api) == 0 || api[0] != "Bearer "+secret {
				t.Errorf("the credential was not used: %v", api)
			}
		})
	}
}

// AC: AC-PLG-003.3
func TestCredentialIsSentOnlyToTheAPI(t *testing.T) {
	h := newHarness(t)
	h.env["GH_TOKEN"] = secret
	h.api.publish(t, pinned, "release_0.2.0_linux_amd64.tar.gz")

	h.succeed()
	api, asset, _ := h.api.requests()
	if len(asset) != 2 {
		t.Fatalf("made %d asset-host requests, want 2 (redirects followed)", len(asset))
	}
	for i, auth := range asset {
		if auth != "" {
			t.Errorf("asset-host request %d carried Authorization %q", i, auth)
		}
	}
	for i, auth := range api {
		if auth != "Bearer "+secret {
			t.Errorf("API request %d sent Authorization %q", i, auth)
		}
	}
}

func TestMissingDataDirIsAUsageError(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{}, {""}} {
		cmd := exec.Command("bash", append([]string{h.script}, args...)...)
		cmd.Env = []string{"PATH=" + h.toolDir}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
			t.Errorf("args %q: err = %v, want exit 2", args, err)
		}
		if !strings.HasPrefix(stderr.String(), "usage: ") {
			t.Errorf("args %q: stderr = %q, want a usage line", args, stderr.String())
		}
	}
}

// The script reads the version from the plugin it ships in.
func TestPinnedVersionIsThePluginManifestVersion(t *testing.T) {
	h := newHarness(t)
	h.pin("0.3.1")
	onPath := h.tool("release", string(fakeRelease("0.3.1")))
	if path := h.succeed(); path != onPath {
		t.Errorf("path = %s, want the release at the manifest's 0.3.1", path)
	}
}

// writeManifest replaces the plugin manifest the script reads.
func (h *harness) writeManifest(body string) {
	h.t.Helper()
	mustWrite(h.t, filepath.Join(h.root, "plugin", ".claude-plugin", "plugin.json"), []byte(body), 0o644)
}

// Only the top-level version pins the CLI, wherever a nested one sits and
// however the file is laid out.
func TestOnlyTheTopLevelManifestVersionPins(t *testing.T) {
	cases := map[string]string{
		"nested before and after": `{
  "name": "release",
  "meta": {"version": "9.9.9"},
  "version": "0.3.1",
  "dependencies": [{"name": "specs", "version": "1.4.0"}],
  "tail": {"x": {"version": "8.8.8"}}
}
`,
		"minified, nested after":  `{"name":"release","version":"0.3.1","dependencies":[{"name":"specs","version":"1.4.0"}]}`,
		"minified, nested before": `{"meta":{"version":"9.9.9"},"version":"0.3.1"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.writeManifest(body)
			onPath := h.tool("release", string(fakeRelease("0.3.1")))
			if path := h.succeed(); path != onPath {
				t.Errorf("path = %s, want the release at the top-level 0.3.1", path)
			}
		})
	}
}

// A manifest with no top-level version, or two, is the one-line failure.
func TestManifestWithoutExactlyOneTopLevelVersionFails(t *testing.T) {
	h := newHarness(t)
	h.writeManifest(`{"name":"release","meta":{"version":"9.9.9"}}`)
	if got := h.fail(); !strings.Contains(got, "no version in ") {
		t.Errorf("stderr = %q, want %q", got, "no version in ")
	}
	h.writeManifest(`{"version":"0.3.1","name":"release","version":"0.3.2"}`)
	if got := h.fail(); !strings.Contains(got, "more than one top-level version") {
		t.Errorf("stderr = %q, want the duplicate-version failure", got)
	}
}

// snapshot records every path under root except skip, with what would show
// a change to it: a created or removed entry, or a changed file.
func snapshot(t *testing.T, root, skip string) map[string]string {
	t.Helper()
	out := map[string]string{}
	walk(t, root, func(rel string, info fs.FileInfo) {
		p := filepath.Join(root, rel)
		if p == skip || strings.HasPrefix(p, skip+string(filepath.Separator)) {
			return
		}
		if info.IsDir() {
			// A directory's mtime moves when the data directory is created
			// in it; a change to its entries shows as an entry of its own.
			out[rel] = info.Mode().String()
			return
		}
		out[rel] = fmt.Sprintf("%v %d %d", info.Mode(), info.Size(), info.ModTime().UnixNano())
	})
	return out
}

func walk(t *testing.T, root string, fn func(rel string, info fs.FileInfo)) {
	t.Helper()
	err := filepath.Walk(root, func(p string, info fs.FileInfo, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		fn(rel, info)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}
