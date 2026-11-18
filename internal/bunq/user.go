package bunq

import "context"

// User is the logged-in bunq user.
type User struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
}

// User returns the user with the given id.
func (c *Client) User(ctx context.Context, userID int) (User, error) {
	return get[User](ctx, c, apiPath("user", userID), "UserPerson", "UserCompany", "UserApiKey")
}
