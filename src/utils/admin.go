package utils

import (
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

func Sudo(c *command.Ctx) {
	c.DeleteInvocation()
	c.Client.Rest.SendTyping(c.ChannelID)
	c.Client.Rest.CreateMessage(c.ChannelID, discord.MessageCreate{
		Content:         c.String("text"),
		AllowedMentions: &discord.AllowedMentions{},
	})
}

func LeaveServer(c *command.Ctx) {
	c.ReplyPrivate("Bye! 👋")
	c.Client.Rest.LeaveGuild(c.GuildID)
}

func Help(c *command.Ctx) {
	prefix := config.GetConfig().BotPrefix
	var sb strings.Builder
	for _, cmd := range command.All() {
		if cmd.OwnerOnly {
			continue
		}
		sb.WriteString("`" + prefix + cmd.Name + "`")
		if len(cmd.Aliases) > 0 {
			sb.WriteString(" (" + strings.Join(cmd.Aliases, ", ") + ")")
		}
		sb.WriteString(" - " + cmd.Description + "\n")
	}

	c.ReplyEmbed(discord.Embed{
		Title:       "Commands",
		Description: sb.String(),
		Footer: &discord.EmbedFooter{
			Text: "Every command also works as a slash command. Right click a message → Apps for more!",
		},
	})
}
