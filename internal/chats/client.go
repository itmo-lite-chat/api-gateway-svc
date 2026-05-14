package chats

import (
	"context"
	"time"

	"github.com/itmo-lite-chat/api-gateway-svc/internal/domain"
	pb "github.com/itmo-lite-chat/proto-registry/gen/services/chats_service/chats/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.ChatsServiceClient
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{conn: conn, client: pb.NewChatsServiceClient(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) GetOrCreatePrivate(ctx context.Context, ownerID, participantID string) (domain.StoredChat, error) {
	resp, err := c.client.GetOrCreatePrivateChat(ctx, &pb.GetOrCreatePrivateChatRequest{
		OwnerId:       ownerID,
		ParticipantId: participantID,
	})
	if err != nil {
		return domain.StoredChat{}, err
	}

	return fromProto(resp.GetChat(), resp.GetMembers()), nil
}

func (c *Client) List(ctx context.Context, userID string) ([]domain.StoredChat, error) {
	resp, err := c.client.ListUserChats(ctx, &pb.ListUserChatsRequest{UserId: userID})
	if err != nil {
		return nil, err
	}

	chats := make([]domain.StoredChat, 0, len(resp.GetChats()))
	for _, chat := range resp.GetChats() {
		details, err := c.client.GetChatDetails(ctx, &pb.GetChatDetailsRequest{ChatId: chat.GetChatId()})
		if err != nil {
			return nil, err
		}
		chats = append(chats, fromProto(details.GetChat(), details.GetMembers()))
	}

	return chats, nil
}

func (c *Client) Get(ctx context.Context, chatID string) (domain.StoredChat, error) {
	resp, err := c.client.GetChatDetails(ctx, &pb.GetChatDetailsRequest{ChatId: chatID})
	if err != nil {
		return domain.StoredChat{}, err
	}

	return fromProto(resp.GetChat(), resp.GetMembers()), nil
}

func (c *Client) CheckMember(ctx context.Context, chatID, userID string) (bool, error) {
	resp, err := c.client.CheckChatMember(ctx, &pb.CheckChatMemberRequest{ChatId: chatID, UserId: userID})
	if err != nil {
		return false, err
	}
	return resp.GetIsMember(), nil
}

func (c *Client) TouchLastMessage(ctx context.Context, chatID string, messageID int64, preview string) error {
	_, err := c.client.TouchChatLastMessage(ctx, &pb.TouchChatLastMessageRequest{
		ChatId:             chatID,
		LastMessageId:      messageID,
		LastMessagePreview: preview,
	})
	return err
}

func fromProto(chat *pb.Chat, members []*pb.ChatMember) domain.StoredChat {
	if chat == nil {
		return domain.StoredChat{}
	}

	lastMessageAt := time.Now()
	if chat.GetLastMessageAt() != nil {
		lastMessageAt = chat.GetLastMessageAt().AsTime()
	} else if chat.GetUpdatedAt() != nil {
		lastMessageAt = chat.GetUpdatedAt().AsTime()
	}

	participants := make([]string, 0, len(members))
	for _, member := range members {
		participants = append(participants, member.GetUserId())
	}

	return domain.StoredChat{
		ID:            chat.GetChatId(),
		Participants:  participants,
		LastMessage:   chat.GetLastMessagePreview(),
		LastMessageAt: lastMessageAt,
	}
}
