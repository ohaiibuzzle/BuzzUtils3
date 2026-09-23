package birthdays

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/ncruces/go-sqlite3/driver"
)

var databasePath = "runtime/birthdays.db"
var bday_flagfile = "runtime/birthdays.flag"

var (
	birthdayCoroutineRunning bool
	birthdayCoroutineMu      sync.Mutex
	birthdayStopChan         chan struct{}
)

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, err
	}

	// Create the birthdays channel mapping for each guild if it doesn't exist
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS BirthdayMessage ([GuildID] INTEGER PRIMARY KEY, [ChannelID] Integer)
	`)
	if err != nil {
		return nil, err
	}

	// Create the birthdays table if it doesn't exist
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS Birthdays ([MemberID] INTEGER, [GuildID] INTEGER, [Birthday] TEXT, PRIMARY KEY ([MemberID],[GuildID]))
	`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

type Birthday struct {
	ID        int
	UserID    string
	GuildID   string
	Birthdate string
}

func SetGuildChannel(guildID string, channelID string) error {
	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Insert or update the channel for the guild
	_, err = db.Exec(`
	INSERT INTO BirthdayMessage (GuildID, ChannelID) VALUES (?, ?)
	ON CONFLICT(GuildID) DO UPDATE SET ChannelID=excluded.ChannelID
	`, guildID, channelID)
	if err != nil {
		return err
	}

	return nil
}

func GetGuildChannel(guildID string) (string, error) {
	db, err := initDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	var channelID string
	err = db.QueryRow("SELECT ChannelID FROM BirthdayMessage WHERE GuildID = ?", guildID).Scan(&channelID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No channel set for this guild
		}
		return "", err
	}

	return channelID, nil
}

func (b *Birthday) SetBirthday(guildID string, userID string, birthdate string) error {
	if _, err := time.Parse("2006-01-02", birthdate); err != nil {
		return err
	}

	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Insert or update the birthday for the user in the guild
	_, err = db.Exec(`
	INSERT INTO Birthdays (MemberID, GuildID, Birthday) VALUES (?, ?, ?)
	ON CONFLICT(MemberID, GuildID) DO UPDATE SET Birthday=excluded.Birthday
	`, userID, guildID, birthdate)
	if err != nil {
		return err
	}

	return nil
}

func (b *Birthday) GetBirthday(guildID string, userID string) (string, error) {
	db, err := initDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	var birthdate string
	err = db.QueryRow("SELECT Birthday FROM Birthdays WHERE MemberID = ? AND GuildID = ?", userID, guildID).Scan(&birthdate)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No birthday set for this user in this guild
		}
		return "", err
	}

	return birthdate, nil
}

func (b *Birthday) GetAllBirthdays(guildID string) ([]Birthday, error) {
	db, err := initDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT MemberID, Birthday FROM Birthdays WHERE GuildID = ?", guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []Birthday
	for rows.Next() {
		var b Birthday
		b.GuildID = guildID
		err := rows.Scan(&b.UserID, &b.Birthdate)
		if err != nil {
			return nil, err
		}
		birthdays = append(birthdays, b)
	}

	return birthdays, nil
}

func (b *Birthday) DeleteBirthday(guildID string, userID string) error {
	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM Birthdays WHERE MemberID = ? AND GuildID = ?", userID, guildID)
	if err != nil {
		return err
	}

	return nil
}

func StartBirthdayCoroutine(ctx *discordgo.Session) {
	birthdayCoroutineMu.Lock()
	if birthdayCoroutineRunning {
		birthdayCoroutineMu.Unlock()
		return // Already running
	}
	birthdayCoroutineRunning = true
	stop := make(chan struct{})
	birthdayStopChan = stop
	birthdayCoroutineMu.Unlock()

	go func() {
		for {
			// Get the current date
			now := time.Now()
			currentDate := now.Format("2006-01-02")

			// Read the flagfile to check if the coroutine has already run today
			flagData, err := os.ReadFile(bday_flagfile)
			if err == nil && strings.Contains(string(flagData), currentDate) {
				// do nothing.
			} else {
				// Get all guilds the bot is in
				guilds, err := ctx.UserGuilds(200, "", "", false)
				if err != nil {
					log.Default().Println("Error fetching guilds: " + err.Error())
				} else {
					for _, guild := range guilds {
						// Get all birthdays for the guild
						birthdays, err := (&Birthday{}).GetAllBirthdays(guild.ID)
						if err != nil {
							log.Default().Println("Error fetching birthdays for guild " + guild.ID + ": " + err.Error())
							continue
						}

						// Check if any birthdays match today's month and day, regardless of birth year
						for _, birthday := range birthdays {
							bDate, err := time.Parse("2006-01-02", birthday.Birthdate)
							if err != nil {
								log.Default().Println("Error parsing birthday for user " + birthday.UserID + ": " + err.Error())
								continue
							}
							if bDate.Month() != now.Month() || bDate.Day() != now.Day() {
								continue
							}

							// Get the channel to send the birthday message
							channelID, err := GetGuildChannel(guild.ID)
							if err != nil {
								log.Default().Println("Error fetching channel for guild " + guild.ID + ": " + err.Error())
								continue
							}
							if channelID == "" {
								log.Default().Println("No birthday channel set for guild " + guild.ID)
								continue
							}

							// Send the birthday message
							_, err = ctx.ChannelMessageSend(channelID, "Happy Birthday <@"+birthday.UserID+">! 🎉")
							if err != nil {
								log.Default().Println("Error sending birthday message in guild " + guild.ID + ": " + err.Error())
								continue
							}
						}
					}

					// Writes flagfile to indicate that the coroutine has ran today
					err = os.WriteFile(bday_flagfile, []byte(currentDate), 0644)
					if err != nil {
						log.Default().Println("Error writing flagfile: " + err.Error())
					}
				}
			}

			// Sleep for 24 hours before checking again, unless stopped
			select {
			case <-stop:
				return
			case <-time.After(24 * time.Hour):
			}
		}
	}()
}

func StopBirthdayCoroutine() {
	birthdayCoroutineMu.Lock()
	defer birthdayCoroutineMu.Unlock()
	if !birthdayCoroutineRunning {
		return
	}
	close(birthdayStopChan)
	birthdayCoroutineRunning = false
}

// ForgetGuild removes all birthday data for a guild the bot has left.
func ForgetGuild(guildID string) error {
	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err = db.Exec("DELETE FROM Birthdays WHERE GuildID = ?", guildID); err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM BirthdayMessage WHERE GuildID = ?", guildID)
	return err
}
