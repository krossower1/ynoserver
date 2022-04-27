package server

import (
	"strconv"
	"time"

	"github.com/go-co-op/gocron"
)

type RankingCategory struct {
	CategoryId    string               `json:"categoryId"`
	Game          string               `json:"game"`
	SubCategories []RankingSubCategory `json:"subCategories"`
}

type RankingSubCategory struct {
	SubCategoryId string `json:"subCategoryId"`
	Game          string `json:"game"`
	PageCount     int    `json:"pageCount"`
}

type RankingEntryBase struct {
	Position   int     `json:"position"`
	ValueInt   int     `json:"valueInt"`
	ValueFloat float32 `json:"valueFloat"`
}

type RankingEntry struct {
	RankingEntryBase
	Uuid string `json:"uuid"`
}

type Ranking struct {
	RankingEntryBase
	Name       string `json:"name"`
	Rank       int    `json:"rank"`
	Badge      string `json:"badge"`
	SystemName string `json:"systemName"`
}

func StartRankings() {
	s := gocron.NewScheduler(time.UTC)

	var rankingCategories []*RankingCategory

	if len(badges) > 0 {
		badgeCountCategory := &RankingCategory{CategoryId: "badgeCount"}
		rankingCategories = append(rankingCategories, badgeCountCategory)

		badgeCountCategory.SubCategories = append(badgeCountCategory.SubCategories, RankingSubCategory{SubCategoryId: "all"})
		if _, ok := badges[config.gameName]; ok {
			// Badge records needed for determining badge game
			writeGameBadges()
			badgeCountCategory.SubCategories = append(badgeCountCategory.SubCategories, RankingSubCategory{SubCategoryId: config.gameName, Game: config.gameName})
		}
	}

	eventPeriods, err := readEventPeriodData()
	if err != nil {
		writeErrLog("SERVER", "exp", err.Error())
	} else if len(eventPeriods) > 0 {
		expCategory := &RankingCategory{CategoryId: "exp"}
		rankingCategories = append(rankingCategories, expCategory)

		if len(eventPeriods) > 1 {
			expCategory.SubCategories = append(expCategory.SubCategories, RankingSubCategory{SubCategoryId: "all"})
		}
		for _, eventPeriod := range eventPeriods {
			expCategory.SubCategories = append(expCategory.SubCategories, RankingSubCategory{SubCategoryId: strconv.Itoa(eventPeriod.PeriodOrdinal)})
		}

		eventLocationCountCategory := &RankingCategory{CategoryId: "eventLocationCount"}
		rankingCategories = append(rankingCategories, eventLocationCountCategory)

		if len(eventPeriods) > 1 {
			eventLocationCountCategory.SubCategories = append(eventLocationCountCategory.SubCategories, RankingSubCategory{SubCategoryId: "all"})
		}
		for _, eventPeriod := range eventPeriods {
			eventLocationCountCategory.SubCategories = append(eventLocationCountCategory.SubCategories, RankingSubCategory{SubCategoryId: strconv.Itoa(eventPeriod.PeriodOrdinal)})
		}

		freeEventLocationCountCategory := &RankingCategory{CategoryId: "freeEventLocationCount"}
		rankingCategories = append(rankingCategories, freeEventLocationCountCategory)

		if len(eventPeriods) > 1 {
			freeEventLocationCountCategory.SubCategories = append(freeEventLocationCountCategory.SubCategories, RankingSubCategory{SubCategoryId: "all"})
		}
		for _, eventPeriod := range eventPeriods {
			freeEventLocationCountCategory.SubCategories = append(freeEventLocationCountCategory.SubCategories, RankingSubCategory{SubCategoryId: strconv.Itoa(eventPeriod.PeriodOrdinal)})
		}

		eventLocationCompletionCategory := &RankingCategory{CategoryId: "eventLocationCompletion"}
		rankingCategories = append(rankingCategories, eventLocationCompletionCategory)

		if len(eventPeriods) > 1 {
			eventLocationCompletionCategory.SubCategories = append(eventLocationCompletionCategory.SubCategories, RankingSubCategory{SubCategoryId: "all"})
		}
		for _, eventPeriod := range eventPeriods {
			eventLocationCompletionCategory.SubCategories = append(eventLocationCompletionCategory.SubCategories, RankingSubCategory{SubCategoryId: strconv.Itoa(eventPeriod.PeriodOrdinal)})
		}
	}

	for c, category := range rankingCategories {
		err := writeRankingCategory(category.CategoryId, category.Game, c)
		if err != nil {
			writeErrLog("SERVER", category.CategoryId, err.Error())
			continue
		}
		for sc, subCategory := range category.SubCategories {
			err = writeRankingSubCategory(category.CategoryId, subCategory.SubCategoryId, subCategory.Game, sc)
			if err != nil {
				writeErrLog("SERVER", category.CategoryId+"/"+subCategory.SubCategoryId, err.Error())
			}
		}
	}

	s.Every(1).Hour().Do(func() {
		for _, category := range rankingCategories {
			for _, subCategory := range category.SubCategories {
				// Use 2kki server to update 'all' rankings
				if subCategory.SubCategoryId == "all" && config.gameName != "2kki" {
					continue
				}
				err := updateRankingEntries(category.CategoryId, subCategory.SubCategoryId)
				if err != nil {
					writeErrLog("SERVER", category.CategoryId+"/"+subCategory.SubCategoryId, err.Error())
				}
			}
		}
	})

	s.StartAsync()
}
