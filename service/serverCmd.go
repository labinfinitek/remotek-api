package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/lejianwen/rustdesk-api/v2/model"
)

type ServerCmdService struct{}

// List restituisce la pagina page dei comandi del server.
func (is *ServerCmdService) List(page, pageSize uint) (*model.ServerCmdList, error) {
	res := &model.ServerCmdList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.ServerCmd{})
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("conteggio dei comandi del server: %w", err)
	}
	tx.Scopes(Paginate(page, pageSize))
	if err := tx.Find(&res.ServerCmds).Error; err != nil {
		return nil, fmt.Errorf("comandi del server: %w", err)
	}
	return res, nil
}

// Info restituisce la voce id dei comandi del server; ErrNotFound se non
// c'e'.
func (is *ServerCmdService) Info(id uint) (*model.ServerCmd, error) {
	u := &model.ServerCmd{}
	if err := DB.Where("id = ?", id).First(u).Error; err != nil {
		return nil, fmt.Errorf("comando del server %d: %w", id, nonTrovato(err))
	}
	return u, nil
}

// Delete cancella la voce u dei comandi del server.
func (is *ServerCmdService) Delete(u *model.ServerCmd) error {
	return DB.Delete(u).Error
}

// Create salva la voce u dei comandi del server.
func (is *ServerCmdService) Create(u *model.ServerCmd) error {
	res := DB.Create(u).Error
	return res
}

// SendCmd 发送命令
func (is *ServerCmdService) SendCmd(port int, cmd string, arg string) (string, error) {
	// 组装命令
	cmd = cmd + " " + arg
	res, err := is.SendSocketCmd("v6", port, cmd)
	if err == nil {
		return res, nil
	}
	// v6连接失败，尝试v4
	res, err = is.SendSocketCmd("v4", port, cmd)
	if err == nil {
		return res, nil
	}
	return "", err
}

// tempoComandoServer e' il tempo massimo dei comandi al server rustdesk,
// dalla connessione alla risposta: senza, un server che accetta la
// connessione e non risponde terrebbe ferma la richiesta del pannello. I
// test lo accorciano.
var tempoComandoServer = 5 * time.Second

// SendSocketCmd manda cmd al server rustdesk sulla porta port di localhost,
// in IPv6 se ty e' "v6" e in IPv4 se e' "v4", e ne restituisce la risposta.
func (is *ServerCmdService) SendSocketCmd(ty string, port int, cmd string) (string, error) {
	addr := "[::1]"
	tcp := "tcp6"
	if ty == "v4" {
		tcp = "tcp"
		addr = "127.0.0.1"
	}
	scadenza := time.Now().Add(tempoComandoServer)
	ctx, cancel := context.WithDeadline(context.Background(), scadenza)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, tcp, fmt.Sprintf("%s:%v", addr, port))
	if err != nil {
		Logger.Debugf("%s connect to id server failed: %v", ty, err)
		return "", err
	}
	defer conn.Close()
	// La stessa scadenza vale per l'invio e per la risposta.
	if err := conn.SetDeadline(scadenza); err != nil {
		return "", err
	}
	// 发送命令
	_, err = conn.Write([]byte(cmd))
	if err != nil {
		Logger.Debugf("%s send cmd failed: %v", ty, err)
		return "", err
	}
	time.Sleep(100 * time.Millisecond)
	// 读取返回
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err.Error() != "EOF" {
		Logger.Debugf("%s read response failed: %v", ty, err)
		return "", err
	}
	return string(buf[:n]), nil
}

func (is *ServerCmdService) Update(f *model.ServerCmd) error {
	return DB.Model(f).Updates(f).Error
}
