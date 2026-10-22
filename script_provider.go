package sidecar

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ScriptProvider launches an external sxm.py process and treats the HLS server
// it publishes as an opaque media source. The sidecar turns those local HLS
// URLs into ordinary LAN MP3 HTTP streams for STR.
type ScriptProvider struct {
	mu       sync.RWMutex
	stations []Station
	source   map[string]string 
	titles   map[string]string

	python  string
	script  string
	user    string
	pass    string
	hlsHost string
	hlsPort int

	cmd        *exec.Cmd
	cancel     context.CancelFunc
	httpClient *http.Client
}

func NewScriptProvider(username, password string) *ScriptProvider {
	python := strings.TrimSpace(os.Getenv("SXM_PYTHON"))

	if python == "" {
		if os.PathSeparator == '\\' {
			python = "py"
		} else {
			python = "python3"
			if _, err := exec.LookPath(python); err != nil {
				python = "python"
			}
		}
	}

	script := strings.TrimSpace(os.Getenv("SXM_SCRIPT"))
	if script == "" {
		script = "sxm.py"
	}

	hlsHost := strings.TrimSpace(os.Getenv("SXM_HLS_HOST"))
	if hlsHost == "" {
		hlsHost = "127.0.0.1"
	}

	hlsPort := 9998
	if raw := strings.TrimSpace(os.Getenv("SXM_HLS_PORT")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n < 65536 {
			hlsPort = n
		}
	}

	stations := make([]Station, 0, len(StationCatalog))
	source := make(map[string]string, len(StationCatalog))

	for _, c := range StationCatalog {
		stations = append(stations, Station{
			ID:   c.ID,
			Name: c.Name,
		})

		source[c.ID] = fmt.Sprintf(
			"http://%s:%d/%s.m3u8",
			hlsHost,
			hlsPort,
			url.PathEscape(c.SourceID),
		)
	}

	return &ScriptProvider{
		stations:   stations,
		source:     source,
		titles:     make(map[string]string),
		python:     python,
		script:     script,
		user:       username,
		pass:       password,
		hlsHost:    hlsHost,
		hlsPort:    hlsPort,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *ScriptProvider) Start(ctx context.Context) error {
	scriptPath := p.script

	if !filepath.IsAbs(scriptPath) {
		cwd, err := os.Getwd()
		if err == nil {
			scriptPath = filepath.Join(cwd, scriptPath)
		}
	}

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("SXM script not found: %s", scriptPath)
	}

	// sxm.py is responsible for starting its own HLS server.
	//
	// Windows:
	//   py -3 sxm.py USERNAME PASSWORD -p 9998 -s
	//
	// Linux:
	//   python3 sxm.py USERNAME PASSWORD -p 9998 -s
	var cmd *exec.Cmd

	if os.PathSeparator == '\\' && strings.EqualFold(filepath.Base(p.python), "py") {
		cmd = exec.CommandContext(
			ctx,
			p.python,
			"-3.14",
			scriptPath,
			p.user,
			p.pass,
			"-p", strconv.Itoa(p.hlsPort),
			"-s",
		)
	} else {
		cmd = exec.CommandContext(
			ctx,
			p.python,
			scriptPath,
			p.user,
			p.pass,
			"-p", strconv.Itoa(p.hlsPort),
			"-s",
		)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sxm.py: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.cancel = func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	p.mu.Unlock()

	go func() {
		err := cmd.Wait()

		if err != nil {
			fmt.Printf("sxm.py exited: %v\n", err)
		} else {
			fmt.Println("sxm.py exited")
		}
	}()

	time.Sleep(750 * time.Millisecond)

	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return fmt.Errorf("sxm.py exited during startup")
	}

	return nil
}


func (p *ScriptProvider) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *ScriptProvider) Stations() []Station {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Station, len(p.stations))
	copy(out, p.stations)
	return out
}

func (p *ScriptProvider) sourceURL(id string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	u, ok := p.source[id]
	if !ok {
		return "", fmt.Errorf("station %q not found", id)
	}
	return u, nil
}

func (p *ScriptProvider) NowPlaying(id string) (NowPlaying, error) {
	src, err := p.sourceURL(id)
	if err != nil {
		return NowPlaying{}, err
	}

	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return NowPlaying{}, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NowPlaying{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return NowPlaying{}, fmt.Errorf("HLS status %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return NowPlaying{}, err
	}

	title := lastEXTINFTitle(string(data))
	p.mu.Lock()
	p.titles[id] = title
	p.mu.Unlock()
	return NowPlaying{Title: title}, nil
}

func lastEXTINFTitle(playlist string) string {
	scanner := bufio.NewScanner(strings.NewReader(playlist))
	last := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF:") {
			if i := strings.IndexByte(line, ','); i >= 0 && i+1 < len(line) {
				last = strings.TrimSpace(line[i+1:])
			}
		}
	}
	return last
}

func (p *ScriptProvider) Stream(id string, w http.ResponseWriter, r *http.Request) error {
	src, err := p.sourceURL(id)
	if err != nil {
		return err
	}

	args := []string{
		"-hide_banner", "-loglevel", "warning",
		"-i", src,
		"-vn", "-ac", "2", "-ar", "44100",
		"-c:a", "libmp3lame", "-b:a", "256k",
		"-f", "mp3", "pipe:1",
	}
	ffmpeg := strings.TrimSpace(os.Getenv("FFMPEG"))
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if _, err := exec.LookPath(ffmpeg); err != nil {
		return fmt.Errorf("FFmpeg not found: %w", err)
	}

	cmd := exec.CommandContext(r.Context(), ffmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("ffmpeg[%s]: %s\n", id, scanner.Text())
		}
	}()

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	buf := make([]byte, 64*1024)
	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return nil
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}
