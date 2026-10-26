package api

/*
*

	message LoginRequest {
	  string username = 1;
	  bytes password = 2;
	  string my_id = 4;
	  string my_name = 5;
	  OptionMessage option = 6;
	  oneof union {
	    FileTransfer file_transfer = 7;
	    PortForward port_forward = 8;
	  }
	  bool video_ack_required = 9;
	  uint64 session_id = 10;
	  string version = 11;
	  OSLogin os_login = 12;
	  string my_platform = 13;
	  bytes hwid = 14;
	}
*/

type DeviceInfoInLogin struct {
	Name string `json:"name" label:"name"`
	Os   string `json:"os" label:"os"`
	Type string `json:"type" label:"type"`
}

type LoginForm struct {
	AutoLogin  bool              `json:"autoLogin" label:"AutoLogin"`
	DeviceInfo DeviceInfoInLogin `json:"deviceInfo" label:"DeviceInfo"`
	Id         string            `json:"id"  label:"id"`
	Type       string            `json:"type"  label:"type"`
	Uuid       string            `json:"uuid"  label:"uuid"`
	Username   string            `json:"username" validate:"required,gte=2,lte=32" label:"Username"`
	Password   string            `json:"password,omitempty" validate:"gte=4,lte=32" label:"Password"`
}

type UserListQuery struct {
	Page       uint   `json:"page" form:"page" validate:"required" label:"页码"`
	PageSize   uint   `json:"pageSize" form:"pageSize" validate:"required" label:"每页数量"`
	Status     int    `json:"status" form:"status" label:"状态"`
	Accessible string `json:"accessible" form:"accessible"`
}

type PeerListQuery struct {
	Page       uint   `json:"page" form:"page" validate:"required" label:"页码"`
	PageSize   uint   `json:"pageSize" form:"pageSize" validate:"required" label:"每页数量"`
	Status     int    `json:"status" form:"status" label:"状态"`
	Accessible string `json:"accessible" form:"accessible"`
}

// LasciapassareForm e' il PC per cui l'agente AI chiede il lasciapassare
// (POST /api/agente/lasciapassare, ADR-0028), e il modo: assente o
// amministratore come prima, utente senza i permessi di amministratore.
type LasciapassareForm struct {
	Id   string `json:"id" binding:"required,max=100"`
	Modo string `json:"modo" binding:"omitempty,oneof=amministratore utente"`
}

// LasciapassareTecnicoForm e' il PC per cui il tecnico chiede il
// lasciapassare (POST /api/lasciapassare, ADR-0032).
type LasciapassareTecnicoForm struct {
	Id string `json:"id" binding:"required,max=100"`
}
