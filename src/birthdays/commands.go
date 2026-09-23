package birthdays

import (
	"github.com/disgoorg/disgo/discord"
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
				Options: []command.Option{
					{
						Type:        discord.ApplicationCommandOptionTypeString,
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
				Options: []command.Option{
					{
						Type:        discord.ApplicationCommandOptionTypeUser,
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
				Options: []command.Option{
					{
						Type:         discord.ApplicationCommandOptionTypeChannel,
						Name:         "channel",
						Description:  "The channel to post birthday announcements in",
						Required:     true,
						ChannelTypes: []discord.ChannelType{discord.ChannelTypeGuildText},
					},
				},
				Permissions: discord.PermissionManageGuild,
				Handler:     handleChannel,
			},
		},
	})
}

func handleSet(c *command.Ctx) {
	date := c.String("date")

	if err := SetBirthday(c.GuildID, c.Author.ID, date); err != nil {
		c.ReplyPrivate("That doesn't look like a valid date. Please use the YYYY-MM-DD format.")
		return
	}

	c.ReplyPrivate("Your birthday has been set to " + date + ".")
}

func handleRemove(c *command.Ctx) {
	if err := DeleteBirthday(c.GuildID, c.Author.ID); err != nil {
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

	date, err := GetBirthday(c.GuildID, targetID)
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

	mention := discord.UserMention(targetID)
	if date == "" {
		c.ReplyPrivate(mention + " doesn't have a birthday set.")
		return
	}
	c.ReplyPrivate(mention + "'s birthday is " + date + ".")
}

func handleChannel(c *command.Ctx) {
	channel := c.Channel("channel")
	if channel.GuildID() != c.GuildID || channel.Type() != discord.ChannelTypeGuildText {
		c.ReplyPrivate("Please pick a text channel on this server.")
		return
	}

	if err := SetGuildChannel(c.GuildID, channel.ID()); err != nil {
		c.ReplyPrivate("Something went wrong setting the birthday channel.")
		return
	}

	c.ReplyPrivate("Birthday announcements will now be posted in " + channel.Mention() + ".")
}
