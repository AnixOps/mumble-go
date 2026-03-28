package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
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

	go func() {
		http.HandleFunc("/", serveIndex)
		http.HandleFunc("/api/connect", handleConnect)
		http.HandleFunc("/api/disconnect", handleDisconnect)
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

var client *sdk.Client

func handleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, connectResponse{Success: false, Error: err.Error()})
		return
	}

	if client != nil {
		client.Close()
	}

	c := sdk.New(sdk.Config{
		Address:     req.Address,
		Username:    req.Username,
		Password:    req.Password,
		InsecureTLS: req.InsecureTLS,
	})

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

	client = c
	writeJSON(w, connectResponse{
		Success:   true,
		Session:   c.Session(),
		Connected: true,
		Channel:   req.Channel,
	})
}

func handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if client != nil {
		client.Close()
		client = nil
	}
	writeJSON(w, connectResponse{Success: true})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
