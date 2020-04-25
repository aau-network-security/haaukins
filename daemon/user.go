package daemon

import (
	"context"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)


func (d *daemon) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	log.Ctx(ctx).
		Info().
		Str("username", req.Username).
		Msg("login user")

	token, err := d.auth.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}


func (d *daemon) SignupUser(ctx context.Context, req *pb.SignupUserRequest) (*pb.LoginUserResponse, error) {
	log.Ctx(ctx).
		Info().
		Str("username", req.Username).
		Msg("signup user")

	u, err := store.NewUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	k, err := d.users.GetSignupKey(req.Key)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}
	if k.WillBeSuperUser {
		u.SuperUser = true
	}

	if err := d.users.CreateUser(u); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	if err := d.users.DeleteSignupKey(k); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	token, err := d.auth.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}


func (d *daemon) InviteUser(ctx context.Context, req *pb.InviteUserRequest) (*pb.InviteUserResponse, error) {
	log.Ctx(ctx).Info().Msg("invite user")

	u, _ := ctx.Value(us{}).(store.User)
	if !u.SuperUser {
		return &pb.InviteUserResponse{
			Error: "This action requires super user permissions",
		}, nil
	}

	k := store.NewSignupKey()
	if req.SuperUser {
		k.WillBeSuperUser = true
	}

	if err := d.users.CreateSignupKey(k); err != nil {
		return &pb.InviteUserResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.InviteUserResponse{
		Key: k.String(),
	}, nil
}
