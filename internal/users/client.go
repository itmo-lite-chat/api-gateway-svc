package users

import (
	"context"

	"github.com/itmo-lite-chat/api-gateway-svc/internal/domain"
	pb "github.com/itmo-lite-chat/proto-registry/gen/services/users_service/users/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn  *grpc.ClientConn
	users pb.UsersServiceClient
	auth  pb.AuthServiceClient
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:  conn,
		users: pb.NewUsersServiceClient(conn),
		auth:  pb.NewAuthServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Login(ctx context.Context, username, password string) (domain.User, error) {
	resp, err := c.auth.Login(ctx, &pb.LoginRequest{Username: username, Password: password})
	if err != nil {
		return domain.User{}, err
	}

	user := fromProto(resp.GetUser())
	user.Token = resp.GetToken()
	return user, nil
}

func (c *Client) ValidateToken(ctx context.Context, token string) (domain.User, error) {
	resp, err := c.auth.ValidateToken(ctx, &pb.ValidateTokenRequest{Token: token})
	if err != nil {
		return domain.User{}, err
	}

	user := fromProto(resp.GetUser())
	user.Token = token
	return user, nil
}

func (c *Client) GetOrCreateByLogin(ctx context.Context, login string) (domain.User, error) {
	resp, err := c.users.GetOrCreateUserByLogin(ctx, &pb.GetOrCreateUserByLoginRequest{
		Login:    login,
		Username: login,
	})
	if err != nil {
		return domain.User{}, err
	}

	return fromProto(resp.GetUser()), nil
}

func (c *Client) GetByID(ctx context.Context, userID string) (domain.User, error) {
	resp, err := c.users.GetUserByID(ctx, &pb.GetUserByIDRequest{UserId: userID})
	if err != nil {
		return domain.User{}, err
	}

	return fromProto(resp.GetUser()), nil
}

func fromProto(user *pb.User) domain.User {
	if user == nil {
		return domain.User{}
	}

	return domain.User{
		ID:          user.GetUserId(),
		Username:    user.GetLogin(),
		DisplayName: user.GetUsername(),
	}
}
