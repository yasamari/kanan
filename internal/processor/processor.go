package processor

import (
	"fmt"

	"github.com/yasamari/kanan/internal/record"
	"github.com/yasamari/kanan/internal/syoboi"
)

type Processor struct {
	syoboiClient  syoboi.Client
	infoExtractor record.InfoExtractor
}

func New(syoboiClient syoboi.Client, infoExtractor record.InfoExtractor) *Processor {
	return &Processor{
		syoboiClient:  syoboiClient,
		infoExtractor: infoExtractor,
	}
}

func (p *Processor) Process(path string) error {
	broadcastInfo, err := p.infoExtractor.Extract(path)
	if err != nil {
		return fmt.Errorf("failed to extract broadcast info: %w", err)
	}

	program, err := p.getProgramFromSyoboi(broadcastInfo)
	if err != nil {
		return fmt.Errorf("failed to get program from Syoboi: %w", err)
	}

	fmt.Printf("%v\n", program)

	return nil
}
