package imageclassifier

import (
	"github.com/bwmarrin/discordgo"
	tflite "github.com/mattn/go-tflite"
)

var Commands = []string{
	"predict",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "predict":
		PredictCommand(args, msg, ctx)
	}
}

var model *tflite.Model
var interpreter *tflite.Interpreter

func InitializeModel() {
	model = tflite.NewModelFromFile("config/model.tflite")
	interpreter = tflite.NewInterpreter(model, nil)
}

func GetInterpreter() *tflite.Interpreter {
	return interpreter
}

func GetModel() *tflite.Model {
	return model
}
