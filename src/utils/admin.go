package utils

import (
	"fmt"
	"strings"
	"time"

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
	const perPage = 10
	prefix := config.GetConfig().BotPrefix
	var lines []string
	for _, cmd := range command.All() {
		if cmd.OwnerOnly {
			continue
		}
		line := "`" + prefix + cmd.Name + "`"
		if len(cmd.Aliases) > 0 {
			line += " (" + strings.Join(cmd.Aliases, ", ") + ")"
		}
		lines = append(lines, line+" - "+cmd.Description)
	}

	var pages []discord.Embed
	total := (len(lines) + perPage - 1) / perPage
	for i := 0; i < len(lines); i += perPage {
		pages = append(pages, discord.Embed{
			Title:       "Commands",
			Description: strings.Join(lines[i:min(i+perPage, len(lines))], "\n"),
			Footer: &discord.EmbedFooter{
				Text: fmt.Sprintf("Page %d/%d · Every command also works as a slash command. Right click a message → Apps for more!", i/perPage+1, total),
			},
		})
	}
	c.Paginate(pages, 2*time.Minute)
}
