package daemon

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

var (
	NoPrivilegeToDelete     = errors.New("No privilege to delete users ")
	NoPrivilegeToList       = errors.New("No privilege to list users")
	NoUserInformation       = errors.New("No user information retrieved from the request !")
	NoDestroyOnAdmin        = errors.New("An admin account cannot destroy another admin account !")
	NoPrivilegeToChangePass = errors.New("No privilege to change passwd of user ! ")
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

	u, err := store.NewUser(req.Username, req.Name, req.Surname, req.Email, req.Password)
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
	if k.WillBeNPUser {
		u.NPUser = true
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

	if req.NpUser {
		k.WillBeNPUser = true
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

// ListUsers function lists users which are eligible to access commandline and
// webclient of Haaukins, who have credeentials will be displayed
func (d *daemon) ListUsers(ctx context.Context, req *pb.Empty) (*pb.ListUsersResponse, error) {
	var usersResp []*pb.ListUsersResponse_UserInfo
	requester, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return &pb.ListUsersResponse{Users: usersResp, Error: fmt.Sprintf("No logged in user information found error: %v", err)}, err
	}

	if !requester.SuperUser {
		return &pb.ListUsersResponse{Error: NoPrivilegeToList.Error()}, NoPrivilegeToList
	}

	for _, usr := range d.users.ListUsers() {
		usersResp = append(usersResp, &pb.ListUsersResponse_UserInfo{
			Username:    usr.Username,
			Name:        usr.Name,
			Surname:     usr.Username,
			Email:       usr.Email,
			CreatedAt:   usr.CreatedAt.Format(displayTimeFormat),
			IsSuperUser: usr.SuperUser,
			IsNPUser:    usr.NPUser,
		})
	}

	return &pb.ListUsersResponse{Users: usersResp}, nil

}

// DestroyUser function deletes user only admin accounts
// An admin account should not delete another admin account
func (d *daemon) DestroyUser(ctx context.Context, request *pb.DestroyUserRequest) (*pb.DestroyUserResponse, error) {

	requester, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return &pb.DestroyUserResponse{Message: fmt.Sprintf("No logged in user information found error: %v", err)}, err
	}

	if requester.SuperUser {
		if err := d.users.DeleteUserByUsername(request.Username); err != nil {
			log.Error().Msgf("User delete error %v", err)
			return &pb.DestroyUserResponse{Message: fmt.Sprintf("Error on deleting user %s, error: %v", request.Username, err)}, err
		}
		return &pb.DestroyUserResponse{Message: "User " + request.Username + " deleted successfully by " + requester.Username + " !"}, nil
	}
	return &pb.DestroyUserResponse{}, NoPrivilegeToDelete

}

// ChangeUserPasswd provides ability to change user password
func (d *daemon) ChangeUserPasswd(ctx context.Context, request *pb.UpdatePasswdRequest) (*pb.UpdatePasswdResponse, error) {
	// LoginUserRequest is used to update passwd,
	// because it has required fields in it which are username and password.
	requester, err := getUserFromIncomingContext(ctx)
	if err != nil {
		log.Warn().Msgf("User credentials not found ! %v  ", err)
		return &pb.UpdatePasswdResponse{Message: fmt.Sprintf("No logged in user information found error: %v", err)}, err
	}

	// if user is not authenticated for the request
	// return error message to user
	if !requester.SuperUser || requester.NPUser {
		return &pb.UpdatePasswdResponse{Message: NoPrivilegeToChangePass.Error()}, NoPrivilegeToChangePass
	}

	if (request.Username == requester.Username) || requester.SuperUser || !requester.NPUser {
		updateError := d.users.UpdatePasswd(request.Username, request.Password)
		if updateError != nil {
			return &pb.UpdatePasswdResponse{Message: "Error on updating passwd of user: " + updateError.Error()}, updateError
		}
	} else {
		return &pb.UpdatePasswdResponse{Message: NoPrivilegeToChangePass.Error()}, NoPrivilegeToChangePass
	}

	return &pb.UpdatePasswdResponse{Message: "Success"}, nil
}

// getUserFromIncomingContext extracts user information from incoming context
func getUserFromIncomingContext(ctx context.Context) (*store.User, error) {
	u, ok := ctx.Value(us{}).(store.User)
	if !ok {
		log.Error().Msgf("%v", NoUserInformation)
		return &u, NoUserInformation
	}

	return &u, nil

}
