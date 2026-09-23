package birthdays

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	_ "github.com/ncruces/go-sqlite3/driver"
)

const (
	databasePath = "runtime/birthdays.db"
	flagFile     = "runtime/birthdays.flag"
	dateLayout   = "2006-01-02"
)

var (
	db        *sql.DB
	dbOnce    sync.Once
	startOnce sync.Once
)

func getDB() *sql.DB {
	dbOnce.Do(func() {
		var err error
		db, err = sql.Open("sqlite3", databasePath)
		if err != nil {
			log.Fatal("Error opening birthday database: ", err)
		}
		// SQLite only allows one writer; serialise access to avoid SQLITE_BUSY
		db.SetMaxOpenConns(1)
		for _, stmt := range []string{
			`CREATE TABLE IF NOT EXISTS BirthdayMessage ([GuildID] INTEGER PRIMARY KEY, [ChannelID] INTEGER)`,
			`CREATE TABLE IF NOT EXISTS Birthdays ([MemberID] INTEGER, [GuildID] INTEGER, [Birthday] TEXT, PRIMARY KEY ([MemberID],[GuildID]))`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				log.Fatal("Error creating birthday tables: ", err)
			}
		}
	})
	return db
}

func SetGuildChannel(guildID, channelID snowflake.ID) error {
	_, err := getDB().Exec(`INSERT INTO BirthdayMessage (GuildID, ChannelID) VALUES (?, ?)
		ON CONFLICT(GuildID) DO UPDATE SET ChannelID = excluded.ChannelID`, guildID, channelID)
	return err
}

func SetBirthday(guildID, userID snowflake.ID, birthdate string) error {
	if _, err := time.Parse(dateLayout, birthdate); err != nil {
		return err
	}
	_, err := getDB().Exec(`INSERT INTO Birthdays (MemberID, GuildID, Birthday) VALUES (?, ?, ?)
		ON CONFLICT(MemberID, GuildID) DO UPDATE SET Birthday = excluded.Birthday`, userID, guildID, birthdate)
	return err
}

// GetBirthday returns the member's birthday, or "" if they haven't set one.
func GetBirthday(guildID, userID snowflake.ID) (string, error) {
	var birthdate string
	err := getDB().QueryRow(`SELECT Birthday FROM Birthdays WHERE MemberID = ? AND GuildID = ?`, userID, guildID).Scan(&birthdate)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return birthdate, err
}

func DeleteBirthday(guildID, userID snowflake.ID) error {
	_, err := getDB().Exec(`DELETE FROM Birthdays WHERE MemberID = ? AND GuildID = ?`, userID, guildID)
	return err
}

// ForgetGuild removes all birthday data for a guild the bot has left.
func ForgetGuild(guildID snowflake.ID) error {
	if _, err := getDB().Exec(`DELETE FROM Birthdays WHERE GuildID = ?`, guildID); err != nil {
		return err
	}
	_, err := getDB().Exec(`DELETE FROM BirthdayMessage WHERE GuildID = ?`, guildID)
	return err
}

// StartBirthdayCoroutine announces today's birthdays once a day. A flag file records
// the last day announced, so restarts don't announce twice.
func StartBirthdayCoroutine(client *bot.Client) {
	startOnce.Do(func() {
		go func() {
			for {
				announceBirthdays(client, time.Now())
				time.Sleep(24 * time.Hour)
			}
		}()
	})
}

func announceBirthdays(client *bot.Client, now time.Time) {
	today := now.Format(dateLayout)
	if flag, err := os.ReadFile(flagFile); err == nil && strings.Contains(string(flag), today) {
		return
	}

	// Birthdays are stored as YYYY-MM-DD; match on MM-DD regardless of birth year
	rows, err := getDB().Query(`SELECT b.MemberID, m.ChannelID FROM Birthdays b
		JOIN BirthdayMessage m ON b.GuildID = m.GuildID
		WHERE substr(b.Birthday, 6) = ?`, now.Format("01-02"))
	if err != nil {
		log.Default().Println("Error fetching today's birthdays: " + err.Error())
		return
	}
	type announcement struct{ userID, channelID snowflake.ID }
	var todays []announcement
	for rows.Next() {
		var a announcement
		if err := rows.Scan(&a.userID, &a.channelID); err != nil {
			log.Default().Println("Error reading birthday: " + err.Error())
			continue
		}
		todays = append(todays, a)
	}
	rows.Close()

	for _, a := range todays {
		_, err := client.Rest.CreateMessage(a.channelID, discord.MessageCreate{
			Content: "Happy Birthday " + discord.UserMention(a.userID) + "! 🎉",
		})
		if err != nil {
			log.Default().Println("Error sending birthday message in channel " + a.channelID.String() + ": " + err.Error())
		}
	}

	if err := os.WriteFile(flagFile, []byte(today), 0644); err != nil {
		log.Default().Println("Error writing flagfile: " + err.Error())
	}
}
