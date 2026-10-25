package admin

import "github.com/lejianwen/rustdesk-api/v2/model"

type LoginPayload struct {
	Username   string   `json:"username"`
	Email      string   `json:"email"`
	Avatar     string   `json:"avatar"`
	Token      string   `json:"token"`
	RouteNames []string `json:"route_names"`
	Nickname   string   `json:"nickname"`
	// GroupName e' il nome del gruppo dell'utente, per il Profilo del
	// pannello; vuoto se il gruppo non c'e'.
	GroupName string `json:"group_name"`
}

func (lp *LoginPayload) FromUser(user *model.User) {
	lp.Username = user.Username
	lp.Email = user.Email
	lp.Avatar = user.Avatar
	lp.Nickname = user.Nickname
}

type UserOauthItem struct {
	Op     string `json:"op"`
	Status int    `json:"status"`
}

// GroupUsersPayload e' la risposta di groupUsers: quanto serve per scegliere
// il destinatario di una regola di condivisione, e niente di piu'.
type GroupUsersPayload struct {
	Groups []*GroupUsersGroup `json:"groups"`
	Users  []*GroupUsersUser  `json:"users"`
}

type GroupUsersGroup struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type GroupUsersUser struct {
	Id       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	GroupId  uint   `json:"group_id"`
}
