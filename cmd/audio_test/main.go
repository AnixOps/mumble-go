// audio_test is a complete automated audio transmission test system.
// It spawns two bots, has one send audio, and verifies the other receives it.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"mumble-go/client"
	"mumble-go/identity"
	"mumble-go/protocol"
)

var (
	addr        = flag.String("addr", "mumble.hotxiang.cn:64738", "server address")
	user1       = flag.String("user1", "audio-sender", "sender username")
	user2       = flag.String("user2", "audio-receiver", "receiver username")
	pass        = flag.String("pass", "", "password")
	channel     = flag.String("channel", "Root", "channel name")
	testCount   = flag.Int("count", 5, "number of audio tests to run")
	raw         = flag.Bool("raw", true, "use raw codec instead of Opus (for testing)")
	debug       = flag.Bool("debug", false, "enable debug logging")
	identityProfile = flag.String("identity-profile", "", "base identity profile for persistent client certificates")
	identityDir = flag.String("identity-dir", "", "directory for persisted client identities")
	selfRegister = flag.Bool("self-register", false, "attempt self-registration for each bot before testing")
)

type TestResult struct {
	TestNum      int
	Success      bool
	SentBytes    int
	ReceivedBytes int
	Duration     time.Duration
	Error        string
}

// AudioTracker tracks received audio for a bot
type AudioTracker struct {
	mu         sync.Mutex
	samples     [][]byte
	sessionSeen map[uint32]bool
	lastSeq     map[uint32]uint64
	count       atomic.Int32
}

func NewAudioTracker() *AudioTracker {
	return &AudioTracker{
		samples:     make([][]byte, 0),
		sessionSeen: make(map[uint32]bool),
		lastSeq:     make(map[uint32]uint64),
	}
}

func (t *AudioTracker) OnAudio(session uint32, seq uint64, pcm []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.samples = append(t.samples, pcm)
	t.sessionSeen[session] = true
	t.lastSeq[session] = seq
	t.count.Add(1)
}

func (t *AudioTracker) GetCount() int {
	return int(t.count.Load())
}

func (t *AudioTracker) GetTotalBytes() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := 0
	for _, s := range t.samples {
		total += len(s)
	}
	return total
}

// TestBot represents a connected bot instance
type TestBot struct {
	client  *client.Client
	tracker *AudioTracker
	name    string
	session uint32
}

// GenerateTone generates a PCM sine wave tone
func GenerateTone(frequency int, duration time.Duration, sampleRate int, amplitude float64) []byte {
	frameSize := 960 // 20ms at 48kHz
	frames := int(float64(duration.Seconds()) * 50) // 50 frames per second
	samples := frames * frameSize

	pcm := make([]byte, samples*2) // 16-bit samples

	for i := 0; i < samples; i++ {
		sample := int16(amplitude * 32767 * math.Sin(2*math.Pi*float64(frequency)*float64(i)/float64(sampleRate)))
		pcm[i*2] = byte(sample)
		pcm[i*2+1] = byte(sample >> 8)
	}

	return pcm
}

// GenerateSilence generates silent PCM
func GenerateSilence(duration time.Duration, sampleRate int) []byte {
	frameSize := 960
	frames := int(float64(duration.Seconds()) * 50)
	samples := frames * frameSize
	return make([]byte, samples*2)
}

func main() {
	flag.Parse()

	if *debug {
		protocol.EnableDebug = true
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	fmt.Printf("=== Mumble Audio Transmission Test System ===\n")
	fmt.Printf("Server: %s\n", *addr)
	fmt.Printf("Sender: %s\n", *user1)
	fmt.Printf("Receiver: %s\n", *user2)
	fmt.Printf("Channel: %s\n", *channel)
	fmt.Printf("Test count: %d\n\n", *testCount)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	senderName := *user1
	receiverName := *user2
	if *selfRegister && *identityProfile != "" {
		suffix := fmt.Sprintf("-%d", time.Now().Unix()%100000)
		senderName += suffix + "-s"
		receiverName += suffix + "-r"
		fmt.Printf("Using unique registered usernames: %s / %s\n\n", senderName, receiverName)
	}

	// Create two bots
	bot1, err := createBot(senderName, *addr)
	if err != nil {
		log.Fatalf("Failed to create sender bot: %v", err)
	}
	defer bot1.client.Close()

	bot2, err := createBot(receiverName, *addr)
	if err != nil {
		log.Fatalf("Failed to create receiver bot: %v", err)
	}
	defer bot2.client.Close()

	// Wait for both to be ready
	log.Println("Waiting for bots to connect and stabilize...")
	time.Sleep(2 * time.Second)

	if bot1.session == 0 || bot2.session == 0 {
		log.Fatalf("Bots not fully connected: bot1 session=%d, bot2 session=%d", bot1.session, bot2.session)
	}

	log.Printf("Bot1 (sender) connected: session=%d\n", bot1.session)
	log.Printf("Bot2 (receiver) connected: session=%d\n", bot2.session)

	// Print channel info for both bots
	users1 := bot1.client.State().SnapshotUsers()
	users2 := bot2.client.State().SnapshotUsers()
	if u, ok := users1[bot1.session]; ok {
		fmt.Printf("Bot1 (sender) channel: %d user_id=%d cert_hash=%s\n", u.ChannelID, u.UserID, u.CertificateHash)
	}
	if u, ok := users2[bot2.session]; ok {
		fmt.Printf("Bot2 (receiver) channel: %d user_id=%d cert_hash=%s\n", u.ChannelID, u.UserID, u.CertificateHash)
	}

	// If registration is enabled, try to auto-join test channel now that identities exist.
	if *selfRegister {
		fmt.Printf("Attempting automatic join to channel 1 (test) after self-register...\n")
		if err := bot1.client.JoinChannel(1); err != nil {
			fmt.Printf("Bot1 join error: %v\n", err)
		}
		if err := bot2.client.JoinChannel(1); err != nil {
			fmt.Printf("Bot2 join error: %v\n", err)
		}
		time.Sleep(2 * time.Second)
		users1 = bot1.client.State().SnapshotUsers()
		users2 = bot2.client.State().SnapshotUsers()
		if u, ok := users1[bot1.session]; ok {
			fmt.Printf("Bot1 (sender) channel after auto-join: %d user_id=%d cert_hash=%s\n", u.ChannelID, u.UserID, u.CertificateHash)
		}
		if u, ok := users2[bot2.session]; ok {
			fmt.Printf("Bot2 (receiver) channel after auto-join: %d user_id=%d cert_hash=%s\n", u.ChannelID, u.UserID, u.CertificateHash)
		}
	} else {
		fmt.Printf("Waiting 30 seconds for manual move in guest mode...\n")
		for i := 30; i > 0; i-- {
			time.Sleep(1 * time.Second)
			if i%5 == 0 {
				fmt.Printf("  %ds remaining\n", i)
			}
		}
	}


	// Get server info
	fmt.Printf("\n=== Server Info ===\n")
	fmt.Printf("Opus supported: sender=%v receiver=%v\n", bot1.client.SupportsOpus(), bot2.client.SupportsOpus())

	channels := bot1.client.State().SnapshotChannels()
	fmt.Printf("Channels available: %d\n", len(channels))
	for id, ch := range channels {
		fmt.Printf("  [%2d] %s\n", id, ch.Name)
	}

	// Run tests
	fmt.Printf("\n=== Running Audio Transmission Tests ===\n")

	results := make([]TestResult, *testCount)
	var wg sync.WaitGroup

	for i := 0; i < *testCount; i++ {
		wg.Add(1)
		go func(testNum int) {
			defer wg.Done()
			result := runAudioTest(ctx, bot1, bot2, testNum)
			results[testNum] = result
		}(i)
		time.Sleep(500 * time.Millisecond) // Small delay between tests
	}

	wg.Wait()

	// Print results
	fmt.Printf("\n=== Test Results ===\n")
	passed := 0
	for _, r := range results {
		status := "PASS"
		if !r.Success {
			status = "FAIL"
		}
		fmt.Printf("Test %d: %s (sent=%d bytes, received=%d bytes, duration=%v)\n",
			r.TestNum, status, r.SentBytes, r.ReceivedBytes, r.Duration)
		if r.Error != "" {
			fmt.Printf("  Error: %s\n", r.Error)
		}
		if r.Success {
			passed++
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Passed: %d/%d\n", passed, *testCount)
	if passed == *testCount {
		fmt.Printf("ALL TESTS PASSED\n")
		os.Exit(0)
	} else {
		fmt.Printf("SOME TESTS FAILED\n")
		os.Exit(1)
	}
}

func createBot(username, addr string) (*TestBot, error) {
	cfg := client.Config{
		Address:  addr,
		Username: username,
		Password: *pass,
		TLS: &tls.Config{
			ServerName:         "mumble.hotxiang.cn",
			InsecureSkipVerify: true,
		},
		Audio: client.DefaultAudioConfig(),
	}
	if *identityProfile != "" {
		store := identity.NewLocalStore(*identityDir, *identityProfile+"-"+username)
		cfg.Identity = store
		if meta, err := store.Metadata(); err == nil {
			fmt.Printf("[%s] identity\n  sha256=%s\n  sha1=%s\n  subject=%s\n", username, meta.Fingerprint, meta.CertificateSHA1, meta.Subject)
		}
	}

	c := client.New(cfg)
	tracker := NewAudioTracker()
	c.OnAudio(tracker.OnAudio)

	// Use raw codec for testing (when libopus is not available)
	if *raw {
		if err := c.Audio().SetRawCodec(); err != nil {
			return nil, fmt.Errorf("set raw codec: %w", err)
		}
	} else {
		// Use Opus codec
		if err := c.Audio().SetOpusCodec(); err != nil {
			return nil, fmt.Errorf("set opus codec: %w", err)
		}
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.Connect(connectCtx); err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	if *selfRegister {
		if err := c.SelfRegister(); err != nil {
			log.Printf("[%s] self-register failed: %v", username, err)
		} else {
			log.Printf("[%s] self-register request sent", username)
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Wait for session to be assigned
	for i := 0; i < 10; i++ {
		session := c.State().SelfSession()
		if session != 0 {
			return &TestBot{
				client:  c,
				tracker: tracker,
				name:    username,
				session: session,
			}, nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return nil, fmt.Errorf("session not assigned")
}

func runAudioTest(ctx context.Context, sender, receiver *TestBot, testNum int) TestResult {
	result := TestResult{TestNum: testNum}

	// Reset counters
	initialCount := receiver.tracker.GetCount()
	initialBytes := receiver.tracker.GetTotalBytes()

	startTime := time.Now()

	// Generate and send audio
	tone := GenerateTone(440, 500*time.Millisecond, 48000, 0.5) // 440Hz A note, 500ms
	result.SentBytes = len(tone)

	// Send the audio via TCP
	if err := sender.client.SendAudio(tone); err != nil {
		result.Error = fmt.Sprintf("send audio failed: %v", err)
		return result
	}

	// Wait for transmission (up to 2 seconds)
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)

		currentCount := receiver.tracker.GetCount()
		if currentCount > initialCount {
			// Got something!
			result.ReceivedBytes = receiver.tracker.GetTotalBytes() - initialBytes
			result.Duration = time.Since(startTime)
			result.Success = result.ReceivedBytes > 0
			return result
		}
	}

	result.Duration = time.Since(startTime)
	result.ReceivedBytes = receiver.tracker.GetTotalBytes() - initialBytes
	result.Success = result.ReceivedBytes > 0
	if !result.Success {
		result.Error = "timeout waiting for audio reception"
	}

	return result
}
