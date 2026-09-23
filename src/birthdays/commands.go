package birthdays

import (
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func init() {
	command.Register(&command.Command{
		Name:        "birthday",
		Aliases:     []string{"bday"},
		Description: "Manage birthdays and birthday announcements",
		GuildOnly:   true,
		Subcommands: []*command.Command{
			{
				Name:        "set",
				Description: "Set your birthday",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "date",
						Description: "Your birthday, formatted as YYYY-MM-DD",
						Required:    true,
					},
				},
				Handler: handleSet,
			},
			{
				Name:        "remove",
				Description: "Remove your saved birthday",
				Handler:     handleRemove,
			},
			{
				Name:        "get",
				Description: "Look up a saved birthday",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "user",
						Description: "The user to look up (defaults to you)",
						Required:    false,
					},
				},
				Handler: handleGet,
			},
			{
				Name:        "channel",
				Description: "Set the channel birthday announcements are posted in (requires Manage Server)",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionChannel,
						Name:         "channel",
						Description:  "The channel to post birthday announcements in",
						Required:     true,
						ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
					},
				},
				Permissions: discordgo.PermissionManageGuild,
				Handler:     handleChannel,
			},
		},
	})
}

func handleSet(c *command.Ctx) {
	date := c.String("date")

	b := &Birthday{}
	if err := b.SetBirthday(c.GuildID, c.Author.ID, date); err != nil {
		c.ReplyPrivate("That doesn't look like a valid date. Please use the YYYY-MM-DD format.")
		return
	}

	c.ReplyPrivate("Your birthday has been set to " + date + ".")
}

func handleRemove(c *command.Ctx) {
	b := &Birthday{}
	if err := b.DeleteBirthday(c.GuildID, c.Author.ID); err != nil {
		c.ReplyPrivate("Something went wrong removing your birthday.")
		return
	}

	c.ReplyPrivate("Your birthday has been removed.")
}

func handleGet(c *command.Ctx) {
	targetID := c.Author.ID
	self := true
	if user := c.User("user"); user != nil {
		targetID = user.ID
		self = targetID == c.Author.ID
	}

	b := &Birthday{}
	date, err := b.GetBirthday(c.GuildID, targetID)
	if err != nil {
		c.ReplyPrivate("Something went wrong looking up that birthday.")
		return
	}

	if self {
		if date == "" {
			c.ReplyPrivate("You don't have a birthday set.")
			return
		}
		c.ReplyPrivate("Your birthday is " + date + ".")
		return
	}

	mention := "<@" + targetID + ">"
	if date == "" {
		c.ReplyPrivate(mention + " doesn't have a birthday set.")
		return
	}
	c.ReplyPrivate(mention + "'s birthday is " + date + ".")
}

func handleChannel(c *command.Ctx) {
	channel := c.Channel("channel")
	if channel.GuildID != c.GuildID || channel.Type != discordgo.ChannelTypeGuildText {
		c.ReplyPrivate("Please pick a text channel on this server.")
		return
	}

	if err := SetGuildChannel(c.GuildID, channel.ID); err != nil {
		c.ReplyPrivate("Something went wrong setting the birthday channel.")
		return
	}

	c.ReplyPrivate("Birthday announcements will now be posted in <#" + channel.ID + ">.")
}
