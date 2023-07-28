package imageclassifier

import (
	"image"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/nfnt/resize"
	"github.com/ohaiibuzzle/BuzzUtils3/saucefinder"
)

type Predictions struct {
	Drawing float32
	Hentai  float32
	Neutral float32
	Porn    float32
	Sexy    float32
}

func PredictCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	images, err := saucefinder.GetImagesFromMessages(msg.ReferencedMessage)
	if err != nil {
		log.Default().Println("Error getting images: ", err)
		return
	}

	if len(images) == 0 {
		log.Default().Println("No images found")
		return
	}

	firstImage := images[0]

	FromUrl(firstImage)
}

func FromUrl(url string) (string, error) {
	tensor, err := downloadImageToArray(url)

	if err != nil {
		return "", err
	}

	interpreter := GetInterpreter()
	interpreter.AllocateTensors()

	input := interpreter.GetInputTensor(0)
	input.CopyFromBuffer(tensor)
	interpreter.Invoke()

	output := interpreter.GetOutputTensor(0)
	outputData := output.Float32s()

	predictions := Predictions{
		Drawing: outputData[0],
		Hentai:  outputData[1],
		Neutral: outputData[2],
		Porn:    outputData[3],
		Sexy:    outputData[4],
	}

	log.Default().Println(predictions)

	return "test", nil
}

func downloadImageToArray(url string) ([]float32, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Default().Println("Error creating request: ", err)
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Default().Println("Error downloading image: ", err)
		return nil, err
	}

	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Default().Println("Error decoding image: ", err)
		return nil, err
	}

	IMAGE_DIM := 299
	// Resize the image
	resizedImage := resize.Resize(uint(IMAGE_DIM), uint(IMAGE_DIM), img, resize.NearestNeighbor)

	// Convert the image to a tensor (range 0-1)
	tensor := make([]float32, IMAGE_DIM*IMAGE_DIM*3)
	for y := 0; y < IMAGE_DIM; y++ {
		for x := 0; x < IMAGE_DIM; x++ {
			r, g, b, _ := resizedImage.At(x, y).RGBA()
			tensor[(y*IMAGE_DIM+x)*3+0] = float32(r) / 255.0
			tensor[(y*IMAGE_DIM+x)*3+1] = float32(g) / 255.0
			tensor[(y*IMAGE_DIM+x)*3+2] = float32(b) / 255.0
		}
	}

	return tensor, nil
}
