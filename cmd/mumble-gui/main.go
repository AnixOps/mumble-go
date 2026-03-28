package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"mumble-go/sdk"
)

//go:embed web/*
var webFS embed.FS

func main() {
	flag.Parse()

	port := startWebServer()
	openBrowser(fmt.Sprintf("http://127.0.0.1:%d", port))

	log.Printf("Mumble-Go GUI started on http://127.0.0.1:%d", port)
	select {}
}

func startWebServer() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Create temp dir for uploads
	tmpDir, err := os.MkdirTemp("", "mumble-gui-*")
	if err == nil {
		os.Chdir(tmpDir) // Change to temp dir so files are saved there
	}

	go func() {
		http.HandleFunc("/", serveIndex)
		http.HandleFunc("/api/connect", handleConnect)
		http.HandleFunc("/api/disconnect", handleDisconnect)
		http.HandleFunc("/api/play", handlePlay)
		http.HandleFunc("/api/playfile", handlePlayFile)
		http.HandleFunc("/api/upload", handleUpload)
		http.HandleFunc("/api/stop", handleStop)
		http.HandleFunc("/api/status", handleStatus)
		http.Serve(listener, nil)
	}()

	return port
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Run()
}

type connectRequest struct {
	Address     string `json:"address"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Channel     string `json:"channel"`
	InsecureTLS bool   `json:"insecure_tls"`
}

type connectResponse struct {
	Success   bool   `json:"success"`
	Session   uint32 `json:"session,omitempty"`
	Error     string `json:"error,omitempty"`
	Connected bool   `json:"connected"`
	Channel   string `json:"channel,omitempty"`
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

var (
	client *sdk.Client
	player *audioPlayer
	mu     sync.Mutex
)

type audioPlayer struct {
	ctx     context.Context
	cancel  context.CancelFunc
	url     string
	playing bool
}

func (p *audioPlayer) stop() {
	if p != nil && p.cancel != nil {
		p.cancel()
		p.playing = false
	}
}

type playRequest struct {
	URL     string  `json:"url"`
	Volume  float64 `json:"volume"`
}

type statusResponse struct {
	Connected bool   `json:"connected"`
	Playing   bool   `json:"playing"`
	URL       string `json:"url,omitempty"`
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, connectResponse{Success: false, Error: err.Error()})
		return
	}

	mu.Lock()
	if client != nil {
		client.Close()
	}
	if player != nil {
		player.stop()
		player = nil
	}

	c := sdk.New(sdk.Config{
		Address:     req.Address,
		Username:    req.Username,
		Password:    req.Password,
		InsecureTLS: req.InsecureTLS,
	})
	// Configure audio for streaming
	_ = c.ConfigureRawAudio()
	mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		writeJSON(w, connectResponse{Success: false, Error: err.Error()})
		return
	}

	if req.Channel != "" {
		if err := c.JoinChannelByName(req.Channel); err != nil {
			writeJSON(w, connectResponse{Success: false, Error: fmt.Sprintf("join channel: %v", err)})
			return
		}
	}

	mu.Lock()
	client = c
	mu.Unlock()

	writeJSON(w, connectResponse{
		Success:   true,
		Session:   c.Session(),
		Connected: true,
		Channel:   req.Channel,
	})
}

func handleDisconnect(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if player != nil {
		player.stop()
		player = nil
	}
	if client != nil {
		client.Close()
		client = nil
	}
	mu.Unlock()
	writeJSON(w, connectResponse{Success: true})
}

func handlePlay(w http.ResponseWriter, r *http.Request) {
	var req playRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	mu.Lock()
	if client == nil {
		mu.Unlock()
		writeJSON(w, map[string]string{"error": "not connected"})
		return
	}
	if player != nil {
		player.stop()
	}

	// Ensure tools are available
	if err := sdk.EnsureTool("ffmpeg"); err != nil {
		mu.Unlock()
		writeJSON(w, map[string]string{"error": fmt.Sprintf("ffmpeg: %v", err)})
		return
	}
	if req.URL != "" && !isDirectAudioURL(req.URL) {
		if err := sdk.EnsureTool("yt-dlp"); err != nil {
			mu.Unlock()
			writeJSON(w, map[string]string{"error": fmt.Sprintf("yt-dlp: %v", err)})
			return
		}
	}

	playerCtx, cancel := context.WithCancel(context.Background())
	player = &audioPlayer{
		ctx:        playerCtx,
		cancel:     cancel,
		url:        req.URL,
		playing:    true,
	}
	c := client
	mu.Unlock()

	// Stream using SDK's PlayFile or PlayRemote (proven to work)
	go func() {
		var err error
		if isDirectAudioURL(req.URL) {
			err = c.PlayFile(playerCtx, req.URL)
		} else {
			err = c.PlayRemote(playerCtx, req.URL)
		}
		if err != nil && playerCtx.Err() == nil {
			log.Printf("play error: %v", err)
		}
		mu.Lock()
		if player != nil {
			player.playing = false
		}
		mu.Unlock()
	}()

	writeJSON(w, map[string]bool{"playing": true})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	if player != nil {
		player.stop()
		player = nil
	}
	mu.Unlock()
	writeJSON(w, map[string]bool{"playing": false})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	var playing bool
	var url string
	if player != nil {
		playing = player.playing
		url = player.url
	}
	writeJSON(w, statusResponse{
		Connected: client != nil,
		Playing:   playing,
		URL:       url,
	})
}

func isDirectAudioURL(url string) bool {
	if len(url) == 0 {
		return false
	}
	// Only direct audio files need no resolution
	if hasExtension(url, ".mp3", ".wav", ".ogg", ".flac", ".aac", ".m3u8", ".opus") {
		return true
	}
	// m3u8 HLS streams can go directly to ffmpeg
	if hasExtension(url, ".m3u8") {
		return true
	}
	return false
}

func hasExtension(url string, exts ...string) bool {
	for _, ext := range exts {
		if len(url) > len(ext) && url[len(url)-len(ext):] == ext {
			return true
		}
	}
	return false
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, map[string]string{"error": "method not allowed"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("get file: %v", err)})
		return
	}
	defer file.Close()

	// Create temp file
	tmpFile, err := os.CreateTemp("", "audio-*"+filepath.Ext(header.Filename))
	if err != nil {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("create temp: %v", err)})
		return
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("save file: %v", err)})
		return
	}

	writeJSON(w, map[string]string{"path": tmpFile.Name()})
}

type playFileRequest struct {
	Path string `json:"path"`
}

func handlePlayFile(w http.ResponseWriter, r *http.Request) {
	var req playFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	mu.Lock()
	if client == nil {
		mu.Unlock()
		writeJSON(w, map[string]string{"error": "not connected"})
		return
	}
	if player != nil {
		player.stop()
	}

	playerCtx, cancel := context.WithCancel(context.Background())
	player = &audioPlayer{
		ctx:     playerCtx,
		cancel:  cancel,
		url:     req.Path,
		playing: true,
	}
	c := client
	mu.Unlock()

	go func() {
		err := c.PlayFile(playerCtx, req.Path)
		if err != nil && playerCtx.Err() == nil {
			log.Printf("play file error: %v", err)
		}
		mu.Lock()
		if player != nil {
			player.playing = false
		}
		mu.Unlock()
	}()

	writeJSON(w, map[string]bool{"playing": true})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
