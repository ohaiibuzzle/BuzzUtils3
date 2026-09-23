// Package welcome greets new members with a welcome card.
package welcome

import (
	"bytes"
	"database/sql"
	"log"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
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
			Permissions: discord.PermissionAdministrator,
			Handler:     SetupWelcome,
		},
		&command.Command{
			Name:        "clearwelcome",
			Description: "Stop sending welcome messages on this server",
			GuildOnly:   true,
			Permissions: discord.PermissionAdministrator,
			Handler:     ClearWelcome,
		},
	)
}

func SetupWelcome(c *command.Ctx) {
	ch, err := command.FetchChannel(c.Client, c.ChannelID)
	if err != nil || ch.Type() != discord.ChannelTypeGuildText {
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
func OnMemberJoin(e *events.GuildMemberJoin) {
	var channelID snowflake.ID
	err := database.Get().QueryRow(`SELECT ChannelID FROM WelcomeMessage WHERE GuildID = ?`, e.GuildID).Scan(&channelID)
	if err == sql.ErrNoRows {
		return
	}
	if err != nil {
		log.Default().Println("Error fetching welcome channel: " + err.Error())
		return
	}

	user := e.Member.User
	guildName := "the server"
	if guild, ok := e.Client().Caches.Guild(e.GuildID); ok {
		guildName = guild.Name
	}

	embed := discord.Embed{
		Title: "Ding dong! 🔔",
		Fields: []discord.EmbedField{
			command.Field("Member", user.Mention()+" (@"+user.Username+")", false),
			command.Field("Account Creation Date", user.CreatedAt().UTC().Format(time.RFC1123), false),
			command.Field("Enjoy your stay!", "👋", false),
		},
	}
	ms := discord.MessageCreate{}

	img, err := cardimage.Welcome(user, guildName)
	if err != nil {
		log.Default().Println("Error generating welcome image: " + err.Error())
		embed.Thumbnail = &discord.EmbedResource{URL: user.EffectiveAvatarURL(discord.WithSize(256))}
	} else {
		ms.Files = []*discord.File{discord.NewFile("welcome.png", "", bytes.NewReader(img))}
		embed.Image = &discord.EmbedResource{URL: "attachment://welcome.png"}
	}
	ms.Embeds = []discord.Embed{embed}

	if _, err := e.Client().Rest.CreateMessage(channelID, ms); err != nil {
		log.Default().Println("Error sending welcome message: " + err.Error())
	}
}
