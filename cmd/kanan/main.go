package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli/v3"
	syoboi "github.com/yasamari/kanan"
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
		// Found program: 7784, Episode 3
		// Title: 一畳間まんきつ暮らし！
		Action: func(context.Context, *cli.Command) error {
			syoboiClient := syoboi.NewSyoboiClient()

			// BS日テレ 2026/04/27 23:00-23:30 一畳間まんきつ暮らし！ 第3話
			const channelName = "BS日テレ"
			loc, _ := time.LoadLocation("Asia/Tokyo")
			startTime := time.Date(2026, time.April, 27, 23, 00, 00, 00, loc)
			endTime := startTime.Add(30*time.Minute - 1*time.Second)

			// チャンネルIDを取得
			channels, err := syoboiClient.GetChannels()
			if err != nil {
				return fmt.Errorf("failed to get channels: %w", err)
			}
			var channelID int
			for _, ch := range channels {
				if ch.Name == channelName {
					fmt.Printf("Found channel: %s (ID: %d)\n", ch.Name, ch.ID)
					channelID = ch.ID
				}
			}
			if channelID == 0 {
				return fmt.Errorf("channel not found: %s", channelName)
			}

			// チャンネルIDと時間帯から番組を検索
			programs, err := syoboiClient.SearchProgramsByChannelAndTime(channelID, startTime, endTime)
			if err != nil {
				return fmt.Errorf("failed to search programs: %w", err)
			}

			if len(programs) == 0 || len(programs) > 1 {
				fmt.Println("No programs found.")
				return nil
			}

			program := programs[0]
			fmt.Printf("Found program: %d, Episode %d\n", program.TitleID, program.Count)

			// タイトルIDからタイトルを取得
			title, err := syoboiClient.GetTitleByID(int64(program.TitleID))
			if err != nil {
				return fmt.Errorf("failed to get title: %w", err)
			}
			fmt.Printf("Title: %s\n", title.Title)

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
