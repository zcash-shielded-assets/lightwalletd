package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/zcash/lightwalletd/walletrpc"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:9067", grpc.WithInsecure(),
		grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: 10 * time.Second}))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewCompactTxStreamerClient(conn)

	// First, get latest block height
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	latest, err := c.GetLatestBlock(ctx, &pb.ChainSpec{})
	if err != nil {
		log.Fatalf("GetLatestBlock failed: %v", err)
	}
	fmt.Printf("Latest block: %d\n\n", latest.Height)

	// Scan all blocks using GetBlockRange (ORCHARD pool type to trigger FilterTxPool)
	fmt.Println("=== Scanning all blocks via GetBlockRange (ORCHARD) ===")
	totalIssuanceBlocks := 0
	totalIssuances := 0
	emptyBlocks := 0

	stream, err := c.GetBlockRange(ctx, &pb.BlockRange{
		Start:     &pb.BlockID{Height: 1},
		End:       &pb.BlockID{Height: latest.Height},
		PoolTypes: []pb.PoolType{pb.PoolType_ORCHARD},
	})
	if err != nil {
		log.Fatalf("GetBlockRange failed: %v", err)
	}

	for {
		b, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Recv error: %v", err)
		}

		blockIssuances := 0
		for _, tx := range b.Vtx {
			blockIssuances += len(tx.Issuances)
		}
		if blockIssuances > 0 {
			fmt.Printf("Block %d: %d txs, %d issuances\n", b.Height, len(b.Vtx), blockIssuances)
			for ti, tx := range b.Vtx {
				if len(tx.Issuances) > 0 {
					fmt.Printf("  TX %d: %d issuances\n", ti, len(tx.Issuances))
					for ii, iss := range tx.Issuances {
						fmt.Printf("    Issuance %d: assetDescHash=%s finalize=%v ik=%s issuedAmount=%d\n",
							ii, hex.EncodeToString(iss.AssetDescHash),
							iss.Finalize, hex.EncodeToString(iss.Ik), iss.IssuedAmount)
					}
				}
			}
			totalIssuanceBlocks++
			totalIssuances += blockIssuances
		}
		if len(b.Vtx) == 0 {
			emptyBlocks++
		}
	}

	fmt.Printf("\nGetBlockRange (ORCHARD): %d blocks with issuances, %d total issuances\n", totalIssuanceBlocks, totalIssuances)
	fmt.Printf("Empty blocks: %d (no txs matched ORCHARD filter)\n", emptyBlocks)

	// Now check GetBlock for the same blocks to verify consistency
	fmt.Println("\n=== Verifying with GetBlock (no filter) ===")
	if totalIssuanceBlocks > 0 {
		for h := uint64(1); h <= latest.Height; h++ {
			b, err := c.GetBlock(ctx, &pb.BlockID{Height: h})
			if err != nil {
				continue
			}
			for _, tx := range b.Vtx {
				if len(tx.Issuances) > 0 {
					fmt.Printf("GetBlock %d: %d issuances [OK]\n", h, len(tx.Issuances))
				}
			}
		}
	} else {
		fmt.Println("No issuance blocks found at all.")
	}
}
