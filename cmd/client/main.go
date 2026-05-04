package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/adrianliechti/wingman/config"
	"github.com/adrianliechti/wingman/pkg/client"

	"github.com/google/uuid"
)

var (
	errCommandHandled = errors.New("command handled")
	errSwitchModel    = errors.New("switch model")
	errQuit           = errors.New("quit")
)

func main() {
	urlFlag := flag.String("url", "http://localhost:8080", "server url")
	tokenFlag := flag.String("token", "", "server token")
	modelFlag := flag.String("model", "", "model id")

	flag.Parse()

	ctx := context.Background()

	model := *modelFlag

	options := []client.RequestOption{}

	if *tokenFlag != "" {
		options = append(options, client.WithToken(*tokenFlag))
	}

	client := client.New(*urlFlag, options...)

	reader := bufio.NewReader(os.Stdin)
	output := os.Stdout

	printBanner(output, *urlFlag)

	for {
		if model == "" {
			val, err := selectModel(ctx, client, reader, output)

			if err != nil {
				panic(err)
			}

			model = val
		}

		err := runModel(ctx, client, reader, output, model)

		switch {
		case errors.Is(err, errSwitchModel):
			model = ""
			continue

		case errors.Is(err, errQuit):
			return

		case err != nil:
			panic(err)
		}
	}
}

func printBanner(output *os.File, url string) {
	output.WriteString("Wingman client\n")
	output.WriteString("Server: " + url + "\n")
	output.WriteString("Commands: /model, /help, /quit\n")
	output.WriteString("\n")
}

func runModel(ctx context.Context, c *client.Client, reader *bufio.Reader, output *os.File, model string) error {
	modelType := config.DetectModelType(model)

	output.WriteString(fmt.Sprintf("Model: %s", model))

	if modelType != "" {
		output.WriteString(fmt.Sprintf(" (%s)", modelType))
	}

	output.WriteString("\n")
	output.WriteString("\n")

	switch modelType {
	case config.ModelTypeEmbedder:
		return embed(ctx, c, reader, output, model)

	case config.ModelTypeRenderer:
		return render(ctx, c, reader, output, model)

	case config.ModelTypeSynthesizer:
		return synthesize(ctx, c, reader, output, model)

	case config.ModelTypeTranscriber:
		return transcribe(ctx, c, reader, output, model)

	default:
		return chat(ctx, c, reader, output, model)
	}
}

func selectModel(ctx context.Context, client *client.Client, reader *bufio.Reader, output *os.File) (string, error) {
	models, err := client.Models.List(ctx)

	if err != nil {
		return "", err
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	sort.SliceStable(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	output.WriteString("Select model\n")

	for i, m := range models {
		output.WriteString(fmt.Sprintf("%2d) ", i+1))
		output.WriteString(m.ID)
		output.WriteString("\n")
	}

	var idx int

	for {
		output.WriteString("model> ")
		sel, err := reader.ReadString('\n')

		if err != nil {
			return "", err
		}

		idx, err = strconv.Atoi(strings.TrimSpace(sel))

		if err != nil || idx < 1 || idx > len(models) {
			output.WriteString("Invalid selection\n")
			continue
		}

		output.WriteString("\n")
		return models[idx-1].ID, nil
	}
}

func chat(ctx context.Context, c *client.Client, reader *bufio.Reader, output *os.File, model string) error {
	req := client.CompletionRequest{
		Model: model,

		Messages: []client.Message{},

		CompleteOptions: client.CompleteOptions{},
	}

	for {
		output.WriteString("chat> ")
		input, err := reader.ReadString('\n')

		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)

		if strings.EqualFold(input, "/reset") {
			req.Messages = []client.Message{}
			output.WriteString("Conversation reset\n")
			continue
		}

		switch command(input, output) {
		case errSwitchModel:
			return errSwitchModel

		case errQuit:
			return errQuit

		case errCommandHandled:
			continue

		case nil:

		default:
			continue
		}

		if input == "" {
			continue
		}

		req.Messages = append(req.Messages, client.UserMessage(input))

		acc := client.CompletionAccumulator{}
		failed := false

		for c, err := range c.Completions.NewStream(ctx, req) {
			if err != nil {
				output.WriteString(err.Error() + "\n")
				failed = true
				break
			}

			acc.Add(*c)

			if c.Message != nil {
				output.WriteString(c.Message.Text())
			}
		}

		if failed {
			req.Messages = req.Messages[:len(req.Messages)-1]
			output.WriteString("\n")
			continue
		}

		if result := acc.Result(); result.Message != nil {
			req.Messages = append(req.Messages, *result.Message)
		}

		output.WriteString("\n")
		output.WriteString("\n")
	}
}

func embed(ctx context.Context, c *client.Client, reader *bufio.Reader, output *os.File, model string) error {
	for {
		output.WriteString("embed> ")
		input, err := reader.ReadString('\n')

		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)

		if err := command(input, output); errors.Is(err, errSwitchModel) || errors.Is(err, errQuit) {
			return err
		} else if err != nil {
			continue
		}

		if input == "" {
			continue
		}

		result, err := c.Embeddings.New(ctx, client.EmbeddingsRequest{
			Model: model,
			Texts: []string{input},
		})

		if err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		for i, e := range result.Embeddings[0] {
			if i > 0 {
				output.WriteString(", ")
			}

			output.WriteString(fmt.Sprintf("%f", e))
		}

		output.WriteString("\n")
		output.WriteString("\n")
	}
}

func render(ctx context.Context, c *client.Client, reader *bufio.Reader, output *os.File, model string) error {
	for {
		output.WriteString("prompt> ")
		input, err := reader.ReadString('\n')

		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)

		if err := command(input, output); errors.Is(err, errSwitchModel) || errors.Is(err, errQuit) {
			return err
		} else if err != nil {
			continue
		}

		if input == "" {
			continue
		}

		image, err := c.Renderings.New(ctx, client.RenderingRequest{
			Model: model,
			Input: input,
		})

		if err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		name := uuid.New().String()

		name += extByContentType(image.ContentType, ".png")

		if err := os.WriteFile(name, image.Content, 0600); err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		fmt.Println("Saved: " + name)

		output.WriteString("\n")
		output.WriteString("\n")
	}
}

func synthesize(ctx context.Context, c *client.Client, reader *bufio.Reader, output *os.File, model string) error {
	for {
		output.WriteString("text> ")
		input, err := reader.ReadString('\n')

		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)

		if err := command(input, output); errors.Is(err, errSwitchModel) || errors.Is(err, errQuit) {
			return err
		} else if err != nil {
			continue
		}

		if input == "" {
			continue
		}

		synthesis, err := c.Syntheses.New(ctx, client.SynthesizeRequest{
			Model: model,

			Input: input,
		})

		if err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		name := uuid.New().String()

		name += extByContentType(synthesis.ContentType, ".mp3")

		if err := os.WriteFile(name, synthesis.Content, 0600); err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		fmt.Println("Saved: " + name)

		output.WriteString("\n")
		output.WriteString("\n")
	}
}

func transcribe(ctx context.Context, c *client.Client, reader *bufio.Reader, output *os.File, model string) error {
	for {
		output.WriteString("file> ")
		input, err := reader.ReadString('\n')

		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)

		if err := command(input, output); errors.Is(err, errSwitchModel) || errors.Is(err, errQuit) {
			return err
		} else if err != nil {
			continue
		}

		if input == "" {
			continue
		}

		file, err := os.Open(input)

		if err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		result, err := c.Transcriptions.New(ctx, client.TranscribeRequest{
			Model: model,

			Name:   filepath.Base(input),
			Reader: file,
		})

		file.Close()

		if err != nil {
			output.WriteString(err.Error() + "\n")
			continue
		}

		output.WriteString(result.Text)
		output.WriteString("\n")
		output.WriteString("\n")
	}
}

func command(input string, output *os.File) error {
	if !strings.HasPrefix(input, "/") {
		return nil
	}

	switch strings.ToLower(input) {
	case "/help":
		output.WriteString("Commands: /model, /help, /quit")
		output.WriteString("\n")
		return errCommandHandled

	case "/model":
		return errSwitchModel

	case "/quit", "/exit":
		return errQuit
	}

	output.WriteString("Unknown command\n")
	return errCommandHandled
}

var contentTypeExtMap = map[string]string{
	"audio/mpeg": ".mp3",
	"audio/mp3":  ".mp3",
	"audio/wav":  ".wav",
	"audio/pcm":  ".pcm",
	"audio/opus": ".opus",
	"audio/aac":  ".aac",
	"audio/flac": ".flac",
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

func extByContentType(contentType, fallback string) string {
	if ext, ok := contentTypeExtMap[contentType]; ok {
		return ext
	}

	if ext, _ := mime.ExtensionsByType(contentType); len(ext) > 0 {
		return ext[0]
	}

	return fallback
}
