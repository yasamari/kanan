package main

import (
	"context"
	"log"
	"os"

	"github.com/urfave/cli/v3"
	"github.com/yasamari/kanan/internal/processor"
	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/syoboi"
)

func main() {
	cmd := &cli.Command{
		Name:  "kanan",
		Usage: "録画したtsファイルを整理するCLIツール",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "tspath",
				UsageText: "対象のファイルパス",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			syoboiClient := syoboi.NewClient()
			infoExtractor := record.NewTsInfoExtractor()

			err := processor.New(syoboiClient, infoExtractor).Process(cmd.StringArg("tspath"))
			if err != nil {
				return err
			}

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
