package messages

import (
	"context"
	"strconv"
	"time"

	pb "github.com/itmo-lite-chat/proto-registry/gen/services/messages_service/messages/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.MessagesServiceClient
}

type Message struct {
	ID        string
	SenderID  string
	Text      string
	Timestamp time.Time
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: pb.NewMessagesServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Send(ctx context.Context, chatID, senderID, text string) (Message, error) {
	resp, err := c.client.SendMessage(ctx, &pb.SendMessageRequest{
		ChatId:   chatID,
		SenderId: senderID,
		Content: &pb.Content{
			Type: pb.ContentType_CONTENT_TYPE_TEXT,
			Body: text,
		},
	})
	if err != nil {
		return Message{}, err
	}

	return fromProto(resp.GetMessage()), nil
}

func (c *Client) History(ctx context.Context, chatID string) ([]Message, error) {
	resp, err := c.client.GetChatHistory(ctx, &pb.GetChatHistoryRequest{
		ChatId: chatID,
		Limit:  100,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(resp.GetMessages()))
	for _, msg := range resp.GetMessages() {
		messages = append(messages, fromProto(msg))
	}

	return messages, nil
}

func fromProto(msg *pb.Message) Message {
	if msg == nil {
		return Message{}
	}

	var timestamp time.Time
	if msg.GetCreatedAt() != nil {
		timestamp = msg.GetCreatedAt().AsTime()
	}

	return Message{
		ID:        strconv.FormatInt(msg.GetMessageId(), 10),
		SenderID:  msg.GetSenderId(),
		Text:      msg.GetContent().GetBody(),
		Timestamp: timestamp,
	}
}
