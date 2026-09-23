// Package welcome greets new members with a welcome card.
package welcome

import (
	"bytes"
	"database/sql"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/cardimage"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/database"
)

func init() {
	command.Register(
		&command.Command{
			Name:        "setupwelcome",
			Description: "Use this channel for welcome messages",
			GuildOnly:   true,
			Permissions: discordgo.PermissionAdministrator,
			Handler:     SetupWelcome,
		},
		&command.Command{
			Name:        "clearwelcome",
			Description: "Stop sending welcome messages on this server",
			GuildOnly:   true,
			Permissions: discordgo.PermissionAdministrator,
			Handler:     ClearWelcome,
		},
	)
}

func SetupWelcome(c *command.Ctx) {
	ch, err := c.Session.State.Channel(c.ChannelID)
	if err != nil || ch.Type != discordgo.ChannelTypeGuildText {
		c.ReplyPrivate("Cannot use this channel :<")
		return
	}

	_, err = database.Get().Exec(`INSERT INTO WelcomeMessage (GuildID, ChannelID) VALUES (?, ?)
		ON CONFLICT(GuildID) DO UPDATE SET ChannelID = excluded.ChannelID`, c.GuildID, c.ChannelID)
	if err != nil {
		log.Default().Println("Error setting welcome channel: " + err.Error())
		c.ReplyPrivate("Something went wrong setting the welcome channel.")
		return
	}
	c.Reply("Success. This channel will now be used for welcome messages!")
}

func ClearWelcome(c *command.Ctx) {
	if _, err := database.Get().Exec(`DELETE FROM WelcomeMessage WHERE GuildID = ?`, c.GuildID); err != nil {
		log.Default().Println("Error clearing welcome channel: " + err.Error())
		c.ReplyPrivate("Something went wrong clearing the welcome channel.")
		return
	}
	c.Reply("Success. Removed the welcome messages from this server!")
}

// OnMemberJoin sends the welcome card when a member joins a configured guild.
func OnMemberJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	var channelID string
	err := database.Get().QueryRow(`SELECT ChannelID FROM WelcomeMessage WHERE GuildID = ?`, m.GuildID).Scan(&channelID)
	if err == sql.ErrNoRows {
		return
	}
	if err != nil {
		log.Default().Println("Error fetching welcome channel: " + err.Error())
		return
	}

	guildName := "the server"
	if guild, err := s.State.Guild(m.GuildID); err == nil {
		guildName = guild.Name
	}

	embed := &discordgo.MessageEmbed{
		Title: "Ding dong! 🔔",
		Fields: []*discordgo.MessageEmbedField{
			command.Field("Member", m.User.Mention()+" (@"+m.User.Username+")", false),
			command.Field("Account Creation Date", accountCreated(m.User.ID), false),
			command.Field("Enjoy your stay!", "👋", false),
		},
	}
	ms := &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}}

	img, err := cardimage.Welcome(m.User, guildName)
	if err != nil {
		log.Default().Println("Error generating welcome image: " + err.Error())
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: m.User.AvatarURL("256")}
	} else {
		ms.Files = []*discordgo.File{{Name: "welcome.png", ContentType: "image/png", Reader: bytes.NewReader(img)}}
		embed.Image = &discordgo.MessageEmbedImage{URL: "attachment://welcome.png"}
	}

	if _, err := s.ChannelMessageSendComplex(channelID, ms); err != nil {
		log.Default().Println("Error sending welcome message: " + err.Error())
	}
}

func accountCreated(userID string) string {
	created, err := discordgo.SnowflakeTimestamp(userID)
	if err != nil {
		return "Unknown"
	}
	return created.UTC().Format(time.RFC1123)
}
