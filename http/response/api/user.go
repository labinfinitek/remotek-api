package api

import "github.com/lejianwen/rustdesk-api/v2/model"

/*
	pub enum UserStatus {
	    Disabled = 0,
	    Normal = 1,
	    Unverified = -1,
	}
*/

/*
UserPayload e' l'utente come lo legge il client RustDesk (UserPayload in
flutter/lib/common/hbbs/hbbs.dart):

	String name = '';
	String email = '';
	String note = '';
	UserStatus status;
	bool isAdmin = false;
*/
type UserPayload struct {
	Name    string                 `json:"name"`
	Email   string                 `json:"email"`
	Note    string                 `json:"note"`
	IsAdmin *bool                  `json:"is_admin"`
	Status  int                    `json:"status"`
	Info    map[string]interface{} `json:"info"`
}

func (up *UserPayload) FromUser(user *model.User) *UserPayload {
	up.Name = user.Username
	up.Email = user.Email
	up.IsAdmin = user.IsAdmin
	up.Status = int(user.Status)
	up.Info = map[string]interface{}{}
	return up
}

/*
LoginRes e' la risposta di /api/login; Type e' uno dei kAuthRes* della
classe HttpType del client RustDesk:

	class HttpType {
	  static const kAuthReqTypeAccount = "account";
	  static const kAuthReqTypeMobile = "mobile";
	  static const kAuthReqTypeSMSCode = "sms_code";
	  static const kAuthReqTypeEmailCode = "email_code";
	  static const kAuthReqTypeTfaCode = "tfa_code";

	  static const kAuthResTypeToken = "access_token";
	  static const kAuthResTypeEmailCheck = "email_check";
	  static const kAuthResTypeTfaCheck = "tfa_check";
	}
*/
type LoginRes struct {
	Type        string      `json:"type"`
	AccessToken string      `json:"access_token"`
	User        UserPayload `json:"user"`
	Secret      string      `json:"secret,omitempty"`
	TfaType     string      `json:"tfa_type,omitempty"`
}
