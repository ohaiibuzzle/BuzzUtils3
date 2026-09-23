// Package nsfwrole lets members opt in to a server's NSFW role, and lets moderators
// ban members from it.
package nsfwrole

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/database"
)

const (
	confirmTimeout = 15 * time.Second
	optInTimeout   = 60 * time.Second
)

var confirmButtons = []command.Button{
	{Label: "Yes", Style: discordgo.DangerButton, Value: "yes"},
	{Label: "No", Style: discordgo.SecondaryButton, Value: "no"},
}

func init() {
	userOpt := func(description string) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: description,
			Required:    true,
		}
	}
	command.Register(
		&command.Command{
			Name:        "setnsfwrole",
			Description: "Set the NSFW role and enable NSFW opt-in on this server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "role",
					Description: "The role that grants NSFW access",
					Required:    true,
				},
			},
			GuildOnly:   true,
			Permissions: discordgo.PermissionAdministrator,
			Handler:     SetNSFWRole,
		},
		&command.Command{
			Name:        "delnsfwrole",
			Description: "Remove the NSFW role and clear all NSFW bans from this server",
			GuildOnly:   true,
			Permissions: discordgo.PermissionAdministrator,
			Handler:     DelNSFWRole,
		},
		&command.Command{
			Name:        "requestnsfw",
			Description: "Request access to this server's NSFW channels",
			GuildOnly:   true,
			Handler:     RequestNSFW,
		},
		&command.Command{
			Name:        "unnsfw",
			Description: "Remove your NSFW access on this server",
			GuildOnly:   true,
			Handler:     UnNSFW,
		},
		&command.Command{
			Name:        "nsfwban",
			Description: "Ban a member from getting the NSFW role",
			Options: []*discordgo.ApplicationCommandOption{
				userOpt("The member to ban"),
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Why they are being banned",
					Required:    true,
				},
			},
			GuildOnly:   true,
			Permissions: discordgo.PermissionManageRoles,
			Handler:     NSFWBan,
		},
		&command.Command{
			Name:        "nsfwunban",
			Description: "Allow a member to get the NSFW role again",
			Options:     []*discordgo.ApplicationCommandOption{userOpt("The member to unban")},
			GuildOnly:   true,
			Permissions: discordgo.PermissionManageRoles,
			Handler:     NSFWUnban,
		},
	)
}

func SetNSFWRole(c *command.Ctx) {
	role := c.Role("role")
	answer, ok := c.Ask(&discordgo.MessageSend{
		Content:         fmt.Sprintf("I am going to set %s as the server's NSFW role. Confirm?", role.Mention()),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}, c.Author.ID, confirmButtons, confirmTimeout)
	if !ok || answer != "yes" {
		c.Reply("Setup aborted.")
		return
	}

	_, err := database.Get().Exec(`INSERT INTO NSFWRoles (GuildID, RoleID) VALUES (?, ?)
		ON CONFLICT(GuildID) DO UPDATE SET RoleID = excluded.RoleID`, c.GuildID, role.ID)
	if err != nil {
		log.Default().Println("Error setting NSFW role: " + err.Error())
		c.Reply("Something went wrong saving the NSFW role.")
		return
	}
	c.Reply("Done")
}

func DelNSFWRole(c *command.Ctx) {
	answer, ok := c.Ask(&discordgo.MessageSend{
		Content: "I am going to remove the server's NSFW role and clear all NSFW bans. Confirm?",
	}, c.Author.ID, confirmButtons, confirmTimeout)
	if !ok || answer != "yes" {
		c.Reply("Removal aborted.")
		return
	}

	db := database.Get()
	_, err1 := db.Exec(`DELETE FROM NSFWRoles WHERE GuildID = ?`, c.GuildID)
	_, err2 := db.Exec(`DELETE FROM NSFWBans WHERE GuildID = ?`, c.GuildID)
	if err1 != nil || err2 != nil {
		c.Reply("Something went wrong removing the NSFW role.")
		return
	}
	c.Reply("Done")
}

func RequestNSFW(c *command.Ctx) {
	if reason, banned := banReason(c.GuildID, c.Author.ID); banned {
		c.ReplyPrivate("You have been banned from getting NSFW on this server due to: " + reason)
		return
	}
	roleID := nsfwRole(c.GuildID)
	if roleID == "" {
		c.ReplyPrivate("This server does not have a NSFW role set up.")
		return
	}

	guildName := "this server"
	if guild, err := c.Session.State.Guild(c.GuildID); err == nil {
		guildName = guild.Name
	}

	dm, err := c.Session.UserChannelCreate(c.Author.ID)
	if err != nil {
		c.ReplyPrivate("I couldn't DM you. Do you have DMs from server members turned off?")
		return
	}
	c.ReplyPrivate("Please check your DM for instructions.")

	terms := fmt.Sprintf("You are requesting NSFW access to **%[1]s**\n"+
		"By pressing `I agree`, you agree that:\n"+
		"1. You are over the age of consent and are willing to be exposed to NSFW content.\n"+
		"2. You are going have full responsibility for the content you send on %[1]s.\n"+
		"3. You are going to follow Discord's guidelines for NSFW content.\n"+
		"4. If you are caught violating these rules, %[1]s moderators reserves the right to punish you.\n\n"+
		"You have %[2]d seconds to reply to this request or it will be aborted automatically.",
		guildName, int(optInTimeout.Seconds()))

	answer, ok := command.AskInChannel(c.Session, dm.ID, &discordgo.MessageSend{Content: terms}, c.Author.ID, []command.Button{
		{Label: "I agree", Style: discordgo.SuccessButton, Value: "agree"},
		{Label: "Cancel", Style: discordgo.SecondaryButton, Value: "cancel"},
	}, optInTimeout)
	if !ok {
		c.Session.ChannelMessageSend(dm.ID, "This request has been aborted automatically")
		return
	}
	if answer != "agree" {
		c.Session.ChannelMessageSend(dm.ID, "Request cancelled.")
		return
	}

	if err := c.Session.GuildMemberRoleAdd(c.GuildID, c.Author.ID, roleID); err != nil {
		log.Default().Println("Error adding NSFW role: " + err.Error())
		c.Session.ChannelMessageSend(dm.ID, "I couldn't give you the role. Ask a moderator to check my permissions!")
		return
	}
	c.Session.ChannelMessageSend(dm.ID, "Done.")
}

func UnNSFW(c *command.Ctx) {
	roleID := nsfwRole(c.GuildID)
	if roleID == "" {
		c.ReplyPrivate("This server does not have a NSFW role set up.")
		return
	}
	if err := c.Session.GuildMemberRoleRemove(c.GuildID, c.Author.ID, roleID); err != nil {
		log.Default().Println("Error removing NSFW role: " + err.Error())
		c.ReplyPrivate("I couldn't remove the role. Ask a moderator to check my permissions!")
		return
	}
	c.ReplyPrivate("Done.")
}

func NSFWBan(c *command.Ctx) {
	target, reason := c.User("user"), c.String("reason")
	_, err := database.Get().Exec(`INSERT INTO NSFWBans (GuildID, MemberID, BanReason) VALUES (?, ?, ?)
		ON CONFLICT(GuildID, MemberID) DO UPDATE SET BanReason = excluded.BanReason`, c.GuildID, target.ID, reason)
	if err != nil {
		log.Default().Println("Error banning member from NSFW: " + err.Error())
		c.ReplyPrivate("Something went wrong saving the ban.")
		return
	}

	if roleID := nsfwRole(c.GuildID); roleID != "" {
		if err := c.Session.GuildMemberRoleRemove(c.GuildID, target.ID, roleID); err != nil {
			log.Default().Println("Error removing NSFW role: " + err.Error())
		}
	}
	c.Reply(target.Mention() + " has been banned from NSFW")
}

func NSFWUnban(c *command.Ctx) {
	target := c.User("user")
	if _, err := database.Get().Exec(`DELETE FROM NSFWBans WHERE GuildID = ? AND MemberID = ?`, c.GuildID, target.ID); err != nil {
		log.Default().Println("Error unbanning member from NSFW: " + err.Error())
		c.ReplyPrivate("Something went wrong removing the ban.")
		return
	}
	c.Reply("Done.")
}

func nsfwRole(guildID string) string {
	var roleID string
	err := database.Get().QueryRow(`SELECT RoleID FROM NSFWRoles WHERE GuildID = ?`, guildID).Scan(&roleID)
	if err != nil && err != sql.ErrNoRows {
		log.Default().Println("Error fetching NSFW role: " + err.Error())
	}
	return roleID
}

func banReason(guildID, userID string) (string, bool) {
	var reason string
	err := database.Get().QueryRow(`SELECT BanReason FROM NSFWBans WHERE GuildID = ? AND MemberID = ?`, guildID, userID).Scan(&reason)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Default().Println("Error fetching NSFW ban: " + err.Error())
		}
		return "", false
	}
	return reason, true
}
