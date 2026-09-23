// Package database holds the shared server database (welcome channels, marriages,
// NSFW roles). Birthdays keep their own database in the birthdays package.
package database

import (
	"database/sql"
	"log"
	"sync"

	_ "github.com/ncruces/go-sqlite3/driver"
)

var databasePath = "runtime/server_data.db"

var (
	db     *sql.DB
	dbOnce sync.Once
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS WelcomeMessage ([GuildID] INTEGER PRIMARY KEY, [ChannelID] INTEGER)`,
	`CREATE TABLE IF NOT EXISTS Marriage ([ID] INTEGER PRIMARY KEY AUTOINCREMENT, [GuildID] INTEGER,
		[FirstSide] INTEGER, [SecondSide] INTEGER, [StartDate] TEXT)`,
	`CREATE TABLE IF NOT EXISTS NSFWRoles ([GuildID] INTEGER PRIMARY KEY, [RoleID] INTEGER)`,
	`CREATE TABLE IF NOT EXISTS NSFWBans ([GuildID] INTEGER, [MemberID] INTEGER, [BanReason] TEXT,
		PRIMARY KEY ([GuildID], [MemberID]))`,
}

// Get returns the shared database handle, creating the tables on first use.
func Get() *sql.DB {
	dbOnce.Do(func() {
		var err error
		db, err = sql.Open("sqlite3", databasePath)
		if err != nil {
			log.Fatal("Error opening database: ", err)
		}
		// SQLite only allows one writer; serialise access to avoid SQLITE_BUSY
		db.SetMaxOpenConns(1)
		for _, stmt := range schema {
			if _, err := db.Exec(stmt); err != nil {
				log.Fatal("Error creating tables: ", err)
			}
		}
	})
	return db
}

// ForgetMember removes a member's per-guild data when they leave.
func ForgetMember(guildID, userID string) {
	_, err := Get().Exec(`DELETE FROM Marriage WHERE GuildID = ? AND (FirstSide = ? OR SecondSide = ?)`,
		guildID, userID, userID)
	if err != nil {
		log.Default().Println("Error removing member data: " + err.Error())
	}
}

// ForgetGuild removes all data for a guild the bot has left.
func ForgetGuild(guildID string) {
	for _, table := range []string{"WelcomeMessage", "Marriage", "NSFWRoles", "NSFWBans"} {
		if _, err := Get().Exec(`DELETE FROM `+table+` WHERE GuildID = ?`, guildID); err != nil {
			log.Default().Println("Error removing guild data from " + table + ": " + err.Error())
		}
	}
}
