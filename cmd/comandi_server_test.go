package main

import "testing"

// TestTabellaComandiResta prova sul binario vero che i comandi al server tolti
// non toccano un database di prima: la tabella server_cmds, che le versioni
// con i comandi creavano, resta con le sue righe dopo l'avvio, e un database
// nuovo non la crea piu'.
func TestTabellaComandiResta(t *testing.T) {
	dir := sandbox(t)
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("primo avvio: codice %d\n%s", codice, out)
	}
	db := apriDB(t, dir)
	if db.Migrator().HasTable("server_cmds") {
		t.Fatal("il primo avvio ha creato server_cmds")
	}
	// Una tabella con le colonne di model.ServerCmd.
	err := db.Exec("CREATE TABLE `server_cmds` (`id` integer PRIMARY KEY AUTOINCREMENT,`cmd` text NOT NULL DEFAULT ''," +
		"`alias` text NOT NULL DEFAULT '',`option` text NOT NULL DEFAULT '',`explain` text NOT NULL DEFAULT ''," +
		"`target` text NOT NULL DEFAULT '',`created_at` datetime,`updated_at` datetime)").Error
	if err == nil {
		err = db.Exec("INSERT INTO server_cmds (cmd, target) VALUES ('relay-servers', '21115')").Error
	}
	if err != nil {
		t.Fatal(err)
	}
	if codice, out := esegui(t, dir); codice != 0 {
		t.Fatalf("avvio sul database con server_cmds: codice %d\n%s", codice, out)
	}
	var righe int64
	if err := db.Raw("SELECT count(*) FROM server_cmds WHERE cmd = 'relay-servers'").Scan(&righe).Error; err != nil || righe != 1 {
		t.Errorf("server_cmds dopo l'avvio: %d righe (%v), attesa 1", righe, err)
	}
}
