package whatsapp

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"remind-bot/internal/config"
	"remind-bot/internal/phonebook"
	"remind-bot/internal/storage"
	"remind-bot/internal/templates"
)

// CommandHandlerFunc defines the signature for incoming command processors.
type CommandHandlerFunc func(ctx context.Context, chatJID types.JID, senderJID types.JID, args []string) (string, *templates.ReminderMessage, error)

// Bot wraps whatsmeow.Client and handles lifecycle, messaging, and commands.
type Bot struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	cfg       *config.Config
	storage   *storage.Storage
	phonebook *phonebook.Registry

	commands  map[string]CommandHandlerFunc
	cmdMu     sync.RWMutex

	qrCodeStr string
	qrMu      sync.RWMutex

	isConnected bool
	isReady     bool
	readyChan   chan struct{}
	readyOnce   sync.Once
}

// NewBot initializes a new WhatsApp Bot with persistent session store (Turso Cloud or SQLite).
func NewBot(cfg *config.Config, storage *storage.Storage, reg *phonebook.Registry) (*Bot, error) {
	dbLog := waLog.Stdout("Database", cfg.LogLevel, true)
	clientLog := waLog.Stdout("WhatsApp", cfg.LogLevel, true)

	var container *sqlstore.Container
	var err error

	if storage != nil && storage.IsTurso() {
		container = sqlstore.NewWithDB(storage.DB(), "sqlite", dbLog)
		if err := container.Upgrade(context.Background()); err != nil {
			log.Printf("[WhatsApp] Notice: Turso sqlstore direct schema upgrade note: %v. Using local sqlite session.", err)
			dbDSN := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", cfg.DBPath)
			container, err = sqlstore.New(context.Background(), "sqlite", dbDSN, dbLog)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize fallback whatsmeow sqlstore: %w", err)
			}
		} else {
			log.Println("[WhatsApp] Using persistent Turso Cloud for WhatsApp session! ☁️")
		}
	} else {
		dbDSN := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", cfg.DBPath)
		container, err = sqlstore.New(context.Background(), "sqlite", dbDSN, dbLog)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize whatsmeow sqlstore: %w", err)
		}
		log.Printf("[WhatsApp] Using local SQLite session at: %s", cfg.DBPath)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get first device from store: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, clientLog)

	bot := &Bot{
		client:    client,
		container: container,
		cfg:       cfg,
		storage:   storage,
		phonebook: reg,
		commands:  make(map[string]CommandHandlerFunc),
		readyChan: make(chan struct{}),
	}

	client.AddEventHandler(bot.handleEvent)

	return bot, nil
}

// GetLatestQR returns the current live QR code string.
func (b *Bot) GetLatestQR() string {
	b.qrMu.RLock()
	defer b.qrMu.RUnlock()
	return b.qrCodeStr
}

// IsLoggedIn returns true if a paired WhatsApp session exists.
func (b *Bot) IsLoggedIn() bool {
	return b.client != nil && b.client.Store.ID != nil
}

// RegisterCommand registers a bot command handler (e.g. "ping", "jadwal", "jid").
func (b *Bot) RegisterCommand(cmd string, handler CommandHandlerFunc) {
	b.cmdMu.Lock()
	defer b.cmdMu.Unlock()
	b.commands[strings.ToLower(cmd)] = handler
}

// Start connects the WhatsApp client, rendering QR code if unauthenticated.
func (b *Bot) Start(ctx context.Context) error {
	if b.client.Store.ID == nil {
		// No existing session, initiate QR code pairing
		qrChan, err := b.client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}

		if err := b.client.Connect(); err != nil {
			return fmt.Errorf("failed to connect client for QR code: %w", err)
		}

		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					b.qrMu.Lock()
					b.qrCodeStr = evt.Code
					b.qrMu.Unlock()

					fmt.Println()
					fmt.Println("=======================================================")
					fmt.Println("  Scan the QR code below with WhatsApp to pair:")
					fmt.Println("=======================================================")
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
					fmt.Println("=======================================================")
					fmt.Println()
				} else if evt.Event == "success" {
					b.qrMu.Lock()
					b.qrCodeStr = ""
					b.qrMu.Unlock()
					log.Println("[WhatsApp] QR pairing successful! Logged in.")
				} else {
					log.Printf("[WhatsApp] QR Channel Event: %s", evt.Event)
				}
			}
		}()
	} else {
		// Existing session found, connect directly
		log.Printf("[WhatsApp] Connecting with saved session: %s", b.client.Store.ID.String())
		if err := b.client.Connect(); err != nil {
			return fmt.Errorf("failed to connect with existing session: %w", err)
		}
	}

	return nil
}

// WaitReady blocks until the client is connected and ready, or context is cancelled.
func (b *Bot) WaitReady(ctx context.Context) error {
	select {
	case <-b.readyChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop disconnects the client and closes resources.
func (b *Bot) Stop() {
	if b.client != nil && b.client.IsConnected() {
		b.client.Disconnect()
	}
	if b.container != nil {
		_ = b.container.Close()
	}
}

// SendReminder sends a templated reminder message with mention payloads to target JID.
func (b *Bot) SendReminder(ctx context.Context, targetJID string, msg templates.ReminderMessage) error {
	if targetJID == "" {
		targetJID = b.cfg.TargetJID
	}
	if targetJID == "" {
		return fmt.Errorf("no target JID configured (TARGET_JID is empty)")
	}

	jid, err := types.ParseJID(targetJID)
	if err != nil {
		return fmt.Errorf("invalid target JID %q: %w", targetJID, err)
	}

	if !b.client.IsConnected() {
		log.Printf("[WhatsApp] Warning: Client not currently connected, attempting reconnect...")
		if err := b.client.Connect(); err != nil {
			return fmt.Errorf("client not connected and reconnect failed: %w", err)
		}
	}

	protoMsg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(msg.Text),
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: msg.MentionedJID,
			},
		},
	}

	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := b.client.SendMessage(sendCtx, jid, protoMsg)
	if err != nil {
		return fmt.Errorf("failed to send reminder to %s: %w", jid.String(), err)
	}

	log.Printf("[WhatsApp] Successfully sent message to %s (ID: %s, Mentions: %d)", jid.String(), resp.ID, len(msg.MentionedJID))
	return nil
}

// SendSimpleMessage sends a plain text message to a specific JID without mentions.
func (b *Bot) SendSimpleMessage(ctx context.Context, jid types.JID, text string) error {
	protoMsg := &waE2E.Message{
		Conversation: proto.String(text),
	}

	sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	_, err := b.client.SendMessage(sendCtx, jid, protoMsg)
	return err
}

func (b *Bot) handleEvent(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.Connected:
		b.isConnected = true
		log.Printf("[WhatsApp] Connected successfully! Bot JID: %s", b.client.Store.ID.String())
		b.readyOnce.Do(func() {
			b.isReady = true
			close(b.readyChan)
		})

	case *events.LoggedOut:
		b.isConnected = false
		log.Printf("[WhatsApp] Logged out from WhatsApp! Device unlinked.")

	case *events.Disconnected:
		b.isConnected = false
		log.Printf("[WhatsApp] Disconnected from WhatsApp server. Auto-reconnecting will handle reconnect.")

	case *events.Message:
		b.handleIncomingMessage(evt)
	}
}

func (b *Bot) handleIncomingMessage(evt *events.Message) {
	// Ignore own messages
	if evt.Info.IsFromMe {
		return
	}

	text := ExtractMessageText(evt.Message)
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "!") {
		return
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return
	}

	cmdName := strings.TrimPrefix(strings.ToLower(parts[0]), "!")
	args := parts[1:]

	b.cmdMu.RLock()
	handler, exists := b.commands[cmdName]
	b.cmdMu.RUnlock()

	if !exists {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		replyText, reminderMsg, err := handler(ctx, evt.Info.Chat, evt.Info.Sender, args)
		if err != nil {
			log.Printf("[WhatsApp] Command !%s error: %v", cmdName, err)
			_ = b.SendSimpleMessage(ctx, evt.Info.Chat, fmt.Sprintf("⚠️ Error executing !%s: %v", cmdName, err))
			return
		}

		if reminderMsg != nil {
			_ = b.SendReminder(ctx, evt.Info.Chat.String(), *reminderMsg)
		} else if replyText != "" {
			_ = b.SendSimpleMessage(ctx, evt.Info.Chat, replyText)
		}
	}()
}

// ExtractMessageText extracts the textual body from any waE2E.Message variant.
func ExtractMessageText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Conversation != nil && *msg.Conversation != "" {
		return *msg.Conversation
	}
	if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
		return *msg.ExtendedTextMessage.Text
	}
	return ""
}
