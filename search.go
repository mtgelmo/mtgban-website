package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/kodabb/go-mtgban/mtgban"
	"github.com/kodabb/go-mtgban/mtgdb"
	"github.com/kodabb/go-mtgban/mtgjson"
)

func Search(w http.ResponseWriter, r *http.Request) {
	sig := r.FormValue("sig")

	pageVars := genPageNav("Search", sig)

	if !DatabaseLoaded {
		pageVars.Title = "Great things are coming"
		pageVars.ErrorMessage = "Website is starting, please try again in a few minutes"

		render(w, "search.html", pageVars)
		return
	}

	searchParam, _ := GetParamFromSig(sig, "Search")
	canSearch, _ := strconv.ParseBool(searchParam)
	if SigCheck && !canSearch {
		pageVars.Title = "This feature is BANned"
		pageVars.ErrorMessage = ErrMsgPlus
		pageVars.ShowPromo = true

		render(w, "search.html", pageVars)
		return
	}

	query := r.FormValue("q")

	if query != "" {
		pageVars.SearchQuery = query
		pageVars.CondKeys = []string{"TCG", "NM", "SP", "MP", "HP", "PO"}
		pageVars.FoundSellers = map[mtgdb.Card]map[string][]mtgban.CombineEntry{}
		pageVars.FoundVendors = map[mtgdb.Card][]mtgban.CombineEntry{}
		pageVars.Images = map[mtgdb.Card]string{}

		filterEdition := ""
		filterCondition := ""
		if strings.Contains(query, "s:") {
			fields := strings.Fields(query)
			for _, field := range fields {
				if strings.HasPrefix(field, "s:") {
					query = strings.TrimPrefix(query, field)
					query = strings.TrimSuffix(query, field)
					query = strings.TrimSpace(query)

					code := strings.TrimPrefix(field, "s:")
					filterEdition, _ = mtgdb.EditionCode2Name(code)
					break
				}
			}
		}
		if strings.Contains(query, "c:") {
			fields := strings.Fields(query)
			for _, field := range fields {
				if strings.HasPrefix(field, "c:") {
					query = strings.TrimPrefix(query, field)
					query = strings.TrimSuffix(query, field)
					query = strings.TrimSpace(query)

					filterEdition = strings.TrimPrefix(field, "c:")
					break
				}
			}
		}

		for _, seller := range Sellers {
			inventory, err := seller.Inventory()
			if err != nil {
				log.Println(err)
				continue
			}
			for card, entries := range inventory {
				if mtgjson.NormPrefix(card.Name, query) {
					if filterEdition != "" && filterEdition != card.Edition {
						continue
					}

					if pageVars.Images[card] == "" {
						code, _ := mtgdb.EditionName2Code(card.Edition)
						link := fmt.Sprintf("https://api.scryfall.com/cards/%s/%s?format=image&version=normal", strings.ToLower(code), card.Number)
						pageVars.Images[card] = link
					}

					for _, entry := range entries {
						if filterCondition != "" && filterCondition != entry.Conditions {
							continue
						}

						_, found := pageVars.FoundSellers[card]
						if !found {
							pageVars.FoundSellers[card] = map[string][]mtgban.CombineEntry{}
						}

						conditions := entry.Conditions
						if seller.Info().Name == "TCG Low" {
							conditions = "TCG"
						}
						_, found = pageVars.FoundSellers[card][conditions]
						if !found {
							pageVars.FoundSellers[card][conditions] = []mtgban.CombineEntry{}
						}

						res := mtgban.CombineEntry{
							ScraperName: seller.Info().Name,
							Price:       entry.Price,
							Quantity:    entry.Quantity,
							URL:         entry.URL,
						}
						pageVars.FoundSellers[card][conditions] = append(pageVars.FoundSellers[card][conditions], res)
					}
				}
			}
		}

		for _, vendor := range Vendors {
			buylist, err := vendor.Buylist()
			if err != nil {
				log.Println(err)
				continue
			}
			for card, entry := range buylist {
				if filterEdition != "" && filterEdition != card.Edition {
					continue
				}

				if pageVars.Images[card] == "" {
					code, _ := mtgdb.EditionName2Code(card.Edition)
					link := fmt.Sprintf("https://api.scryfall.com/cards/%s/%s?format=image&version=normal", strings.ToLower(code), card.Number)
					pageVars.Images[card] = link
				}

				if mtgjson.NormPrefix(card.Name, query) {
					_, found := pageVars.FoundVendors[card]
					if !found {
						pageVars.FoundVendors[card] = []mtgban.CombineEntry{}
					}
					res := mtgban.CombineEntry{
						ScraperName: vendor.Info().Name,
						Price:       entry.BuyPrice,
						Ratio:       entry.PriceRatio,
						Quantity:    entry.Quantity,
						URL:         entry.URL,
					}
					pageVars.FoundVendors[card] = append(pageVars.FoundVendors[card], res)
				}
			}
		}

		if len(pageVars.FoundSellers) == 0 && len(pageVars.FoundVendors) == 0 {
			pageVars.InfoMessage = "No results found"
		}
	}

	render(w, "search.html", pageVars)
}
