package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/kodabb/go-mtgban/mtgban"
	"github.com/kodabb/go-mtgban/mtgmatcher"
)

type Sleeper struct {
	CardId string
	Level  int
}

type SleeperEntry struct {
	Meta    []GenericCard
	Letter  string
	BGColor string
}

const (
	MaxSleepers = 34
)

var SleeperLetters = []string{
	"S", "A", "B", "C", "D", "E", "F",
}
var SleeperColors = []string{
	"#ff7f7f", "#ffbf7f", "#ffff7f", "#7fff7f", "#7fbfff", "#7f7fff", "#ff7fff",
}

func Sleepers(w http.ResponseWriter, r *http.Request) {
	sig := getSignatureFromCookies(r)

	pageVars := genPageNav("Sleepers", sig)

	// Load the defaul blocklist (same as Search)
	blocklistRetail, blocklistBuylist := getDefaultBlocklists(sig)

	// Expand with any custom list if necessary
	if Config.SleepersBlockList != nil {
		blocklistRetail = append(blocklistRetail, Config.SleepersBlockList...)
		blocklistBuylist = append(blocklistBuylist, Config.SleepersBlockList...)
	}

	start := time.Now()

	tiers := map[string]int{}

	var tcgSeller mtgban.Seller
	for _, seller := range Sellers {
		if seller != nil && seller.Info().Shorthand == TCG_LOW {
			tcgSeller = seller
			break
		}
	}

	for _, seller := range Sellers {
		if seller == nil {
			continue
		}

		if seller.Info().MetadataOnly {
			continue
		}
		if seller.Info().CountryFlag != "" {
			continue
		}
		if seller.Info().SealedMode {
			continue
		}
		// Skip any seller explicitly in blocklist
		if SliceStringHas(blocklistRetail, seller.Info().Shorthand) {
			continue
		}

		for _, vendor := range Vendors {
			if vendor == nil {
				continue
			}
			if vendor.Info().Shorthand == seller.Info().Shorthand {
				continue
			}
			if vendor.Info().CountryFlag != "" {
				continue
			}

			// Skip any vendor explicitly in blocklist
			if SliceStringHas(blocklistBuylist, vendor.Info().Shorthand) {
				continue
			}

			opts := &mtgban.ArbitOpts{
				MinSpread: MinSpread,
			}

			arbit, err := mtgban.Arbit(opts, vendor, seller)
			if err != nil {
				log.Println(err)
				continue
			}

			// Filter out entries that are invalid
			for i := range arbit {
				if math.Abs(arbit[i].BuylistEntry.PriceRatio) < MaxPriceRatio && arbit[i].Spread < MaxSpread && arbit[i].InventoryEntry.Conditions == "NM" {
					tiers[arbit[i].CardId]++
				}
			}
		}

		if tcgSeller != nil {
			mismatch, err := mtgban.Mismatch(nil, tcgSeller, seller)
			if err != nil {
				log.Println(err)
				continue
			}

			// Filter out entries that are invalid
			for i := range mismatch {
				if mismatch[i].InventoryEntry.Conditions == "NM" {
					tiers[mismatch[i].CardId]++
				}
			}
		}
	}

	results := []Sleeper{}
	for c := range tiers {
		if tiers[c] > 1 {
			results = append(results, Sleeper{
				CardId: c,
				Level:  tiers[c],
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Level > results[j].Level
	})

	// Avoid accessing the first element if empty
	if len(results) == 0 {
		render(w, "sleep.html", pageVars)
		return
	}

	maxrange := float64(len(pageVars.Sleepers) - 1)
	minrange := float64(0)
	exp := float64(minrange - maxrange)
	max := float64(results[0].Level)
	min := float64(results[len(results)-1].Level)

	// Avoid a division by 0
	if max == min {
		pageVars.Title = "Errors have been made"
		pageVars.ErrorMessage = ErrMsgDenied

		render(w, "sleep.html", pageVars)
		return
	}

	for i := range pageVars.Sleepers {
		pageVars.Sleepers[i].Meta = []GenericCard{}
		pageVars.Sleepers[i].Letter = SleeperLetters[i]
		pageVars.Sleepers[i].BGColor = SleeperColors[i]
	}

	for _, res := range results {
		value := float64(res.Level)
		// Normalize between 0,1
		r := (value - min) / (max - min)
		// Scale to the size of the table
		level := int(math.Floor(r*exp) + maxrange)

		if DevMode {
			cc, _ := mtgmatcher.GetUUID(res.CardId)
			log.Println(level, res.Level, cc)
		}

		if level >= len(pageVars.Sleepers) {
			break
		}

		if len(pageVars.Sleepers[level].Meta) > MaxSleepers {
			continue
		}

		pageVars.Sleepers[level].Meta = append(pageVars.Sleepers[level].Meta, uuid2card(res.CardId, true))
	}

	pageVars.Title = "Sleeper cards"

	// Log performance
	user := GetParamFromSig(sig, "UserEmail")
	msg := fmt.Sprintf("Sleepers call by %s took %v", user, time.Since(start))
	Notify("Sleepers", msg)
	LogPages["Sleepers"].Println(msg)
	if DevMode {
		log.Println(msg)
	}

	if DevMode {
		start = time.Now()
	}
	render(w, "sleep.html", pageVars)
	if DevMode {
		log.Println("Sleepers render took", time.Since(start))
	}
}
