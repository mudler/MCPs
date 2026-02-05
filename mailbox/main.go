package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Message represents a single message in the mailbox
type Message struct {
	ID        string    `json:"id"`        // Unique identifier
	Sender    string    `json:"sender"`    // Agent name who sent
	Recipient string    `json:"recipient"` // Agent name recipient
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // When sent
	Read      bool      `json:"read"`      // Read status
}

// Mailbox represents the entire mailbox
type Mailbox struct {
	Messages []Message `json:"messages"`
}

// Input types for different operations
type SendMessageInput struct {
	Recipient string `json:"recipient" jsonschema:"the agent name of the recipient"`
	Content   string `json:"content" jsonschema:"the message content"`
}

type ReadMessagesInput struct{}

type MarkMessageReadInput struct {
	ID string `json:"id" jsonschema:"the ID of the message to mark as read"`
}

type MarkMessageUnreadInput struct {
	ID string `json:"id" jsonschema:"the ID of the message to mark as unread"`
}

type DeleteMessageInput struct {
	ID string `json:"id" jsonschema:"the ID of the message to delete"`
}

// Output types
type SendMessageOutput struct {
	ID        string    `json:"id" jsonschema:"the ID of the sent message"`
	Sender    string    `json:"sender" jsonschema:"the sender agent name"`
	Recipient string    `json:"recipient" jsonschema:"the recipient agent name"`
	Content   string    `json:"content" jsonschema:"the message content"`
	Timestamp time.Time `json:"timestamp" jsonschema:"when the message was sent"`
}

type ReadMessagesOutput struct {
	Messages []Message `json:"messages" jsonschema:"list of messages for this agent"`
	Count    int       `json:"count" jsonschema:"number of messages"`
	Unread   int       `json:"unread" jsonschema:"number of unread messages"`
}

type MarkMessageReadOutput struct {
	Success bool   `json:"success" jsonschema:"whether the operation was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type MarkMessageUnreadOutput struct {
	Success bool   `json:"success" jsonschema:"whether the operation was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

type DeleteMessageOutput struct {
	Success bool   `json:"success" jsonschema:"whether the deletion was successful"`
	Message string `json:"message" jsonschema:"status message"`
}

var mailboxFilePath string
var agentName string

// Generate a unique ID for messages
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// withLock executes a function with file locking
func withLock(filePath string, fn func() error) error {
	lockPath := filePath + ".lock"
	fileLock := flock.New(lockPath)

	// Acquire exclusive lock with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("file is locked by another process")
	}
	defer fileLock.Unlock()

	return fn()
}

// loadMailbox loads the mailbox from file
func loadMailbox() (*Mailbox, error) {
	mailbox := &Mailbox{Messages: []Message{}}

	// Check if file exists
	if _, err := os.Stat(mailboxFilePath); os.IsNotExist(err) {
		// File doesn't exist, return empty mailbox
		return mailbox, nil
	}

	data, err := os.ReadFile(mailboxFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read mailbox file: %w", err)
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, mailbox); err != nil {
			return nil, fmt.Errorf("failed to parse mailbox file: %w", err)
		}
	}

	return mailbox, nil
}

// saveMailbox saves the mailbox to file atomically
func saveMailbox(mailbox *Mailbox) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(mailboxFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file first
	tempFile := mailboxFilePath + ".tmp"
	data, err := json.MarshalIndent(mailbox, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mailbox: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, mailboxFilePath); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// SendMessage sends a message to a recipient agent
func SendMessage(ctx context.Context, req *mcp.CallToolRequest, input SendMessageInput) (
	*mcp.CallToolResult,
	SendMessageOutput,
	error,
) {
	if input.Recipient == "" {
		return nil, SendMessageOutput{}, fmt.Errorf("recipient is required")
	}
	if input.Content == "" {
		return nil, SendMessageOutput{}, fmt.Errorf("content is required")
	}

	var output SendMessageOutput

	err := withLock(mailboxFilePath, func() error {
		mailbox, err := loadMailbox()
		if err != nil {
			return err
		}

		message := Message{
			ID:        generateID(),
			Sender:    agentName,
			Recipient: input.Recipient,
			Content:   input.Content,
			Timestamp: time.Now(),
			Read:      false,
		}

		mailbox.Messages = append(mailbox.Messages, message)

		if err := saveMailbox(mailbox); err != nil {
			return err
		}

		output = SendMessageOutput{
			ID:        message.ID,
			Sender:    message.Sender,
			Recipient: message.Recipient,
			Content:   message.Content,
			Timestamp: message.Timestamp,
		}

		return nil
	})

	if err != nil {
		return nil, SendMessageOutput{}, err
	}

	return nil, output, nil
}

// ReadMessages reads all messages for this agent
func ReadMessages(ctx context.Context, req *mcp.CallToolRequest, input ReadMessagesInput) (
	*mcp.CallToolResult,
	ReadMessagesOutput,
	error,
) {
	var output ReadMessagesOutput

	err := withLock(mailboxFilePath, func() error {
		mailbox, err := loadMailbox()
		if err != nil {
			return err
		}

		// If agent name is empty, return all messages
		var myMessages []Message
		unreadCount := 0
		if agentName == "" {
			// Return all messages
			myMessages = mailbox.Messages
			for _, msg := range mailbox.Messages {
				if !msg.Read {
					unreadCount++
				}
			}
		} else {
			// Filter messages for this agent
			for _, msg := range mailbox.Messages {
				if msg.Recipient == agentName {
					myMessages = append(myMessages, msg)
					if !msg.Read {
						unreadCount++
					}
				}
			}
		}

		output = ReadMessagesOutput{
			Messages: myMessages,
			Count:    len(myMessages),
			Unread:   unreadCount,
		}

		return nil
	})

	if err != nil {
		return nil, ReadMessagesOutput{}, err
	}

	return nil, output, nil
}

// MarkMessageRead marks a message as read
func MarkMessageRead(ctx context.Context, req *mcp.CallToolRequest, input MarkMessageReadInput) (
	*mcp.CallToolResult,
	MarkMessageReadOutput,
	error,
) {
	var output MarkMessageReadOutput

	err := withLock(mailboxFilePath, func() error {
		mailbox, err := loadMailbox()
		if err != nil {
			return err
		}

		found := false
		for i := range mailbox.Messages {
			if mailbox.Messages[i].ID == input.ID {
				// Only allow marking as read if this agent is the recipient
				if mailbox.Messages[i].Recipient != agentName {
					output = MarkMessageReadOutput{
						Success: false,
						Message: fmt.Sprintf("message '%s' does not belong to this agent", input.ID),
					}
					return nil
				}
				mailbox.Messages[i].Read = true
				found = true
				break
			}
		}

		if !found {
			output = MarkMessageReadOutput{
				Success: false,
				Message: fmt.Sprintf("message with ID '%s' not found", input.ID),
			}
			return nil
		}

		if err := saveMailbox(mailbox); err != nil {
			return err
		}

		output = MarkMessageReadOutput{
			Success: true,
			Message: fmt.Sprintf("message '%s' marked as read", input.ID),
		}

		return nil
	})

	if err != nil {
		return nil, MarkMessageReadOutput{}, err
	}

	return nil, output, nil
}

// MarkMessageUnread marks a message as unread
func MarkMessageUnread(ctx context.Context, req *mcp.CallToolRequest, input MarkMessageUnreadInput) (
	*mcp.CallToolResult,
	MarkMessageUnreadOutput,
	error,
) {
	var output MarkMessageUnreadOutput

	err := withLock(mailboxFilePath, func() error {
		mailbox, err := loadMailbox()
		if err != nil {
			return err
		}

		found := false
		for i := range mailbox.Messages {
			if mailbox.Messages[i].ID == input.ID {
				// Only allow marking as unread if this agent is the recipient
				if mailbox.Messages[i].Recipient != agentName {
					output = MarkMessageUnreadOutput{
						Success: false,
						Message: fmt.Sprintf("message '%s' does not belong to this agent", input.ID),
					}
					return nil
				}
				mailbox.Messages[i].Read = false
				found = true
				break
			}
		}

		if !found {
			output = MarkMessageUnreadOutput{
				Success: false,
				Message: fmt.Sprintf("message with ID '%s' not found", input.ID),
			}
			return nil
		}

		if err := saveMailbox(mailbox); err != nil {
			return err
		}

		output = MarkMessageUnreadOutput{
			Success: true,
			Message: fmt.Sprintf("message '%s' marked as unread", input.ID),
		}

		return nil
	})

	if err != nil {
		return nil, MarkMessageUnreadOutput{}, err
	}

	return nil, output, nil
}

// DeleteMessage deletes a message by ID (only if recipient matches agent name)
func DeleteMessage(ctx context.Context, req *mcp.CallToolRequest, input DeleteMessageInput) (
	*mcp.CallToolResult,
	DeleteMessageOutput,
	error,
) {
	var output DeleteMessageOutput

	err := withLock(mailboxFilePath, func() error {
		mailbox, err := loadMailbox()
		if err != nil {
			return err
		}

		found := false
		newMessages := []Message{}
		for _, msg := range mailbox.Messages {
			if msg.ID == input.ID {
				found = true
				// Only allow deletion if this agent is the recipient
				if msg.Recipient != agentName {
					output = DeleteMessageOutput{
						Success: false,
						Message: fmt.Sprintf("message '%s' does not belong to this agent", input.ID),
					}
					return nil
				}
				// Skip this message (delete it)
			} else {
				newMessages = append(newMessages, msg)
			}
		}

		if !found {
			output = DeleteMessageOutput{
				Success: false,
				Message: fmt.Sprintf("message with ID '%s' not found", input.ID),
			}
			return nil
		}

		mailbox.Messages = newMessages

		if err := saveMailbox(mailbox); err != nil {
			return err
		}

		output = DeleteMessageOutput{
			Success: true,
			Message: fmt.Sprintf("message '%s' deleted successfully", input.ID),
		}

		return nil
	})

	if err != nil {
		return nil, DeleteMessageOutput{}, err
	}

	return nil, output, nil
}

func main() {
	// Get file path from environment variable, default to /data/mailbox.json
	mailboxFilePath = os.Getenv("MAILBOX_FILE_PATH")
	if mailboxFilePath == "" {
		mailboxFilePath = "/data/mailbox.json"
	}

	// Get agent name from environment variable (optional - if empty, returns all messages)
	agentName = os.Getenv("MAILBOX_AGENT_NAME")

	// Create directory if it doesn't exist
	os.MkdirAll(filepath.Dir(mailboxFilePath), 0755)

	// Create a server with mailbox tools
	server := mcp.NewServer(&mcp.Implementation{Name: "mailbox", Version: "v1.0.0"}, nil)

	// Register mailbox tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_message",
		Description: "Send a message to a recipient agent",
	}, SendMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_messages",
		Description: "Read all messages for this agent (or all messages if agent name is empty)",
	}, ReadMessages)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mark_message_read",
		Description: "Mark a message as read by ID",
	}, MarkMessageRead)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mark_message_unread",
		Description: "Mark a message as unread by ID",
	}, MarkMessageUnread)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_message",
		Description: "Delete a message by ID (only if recipient matches this agent)",
	}, DeleteMessage)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
