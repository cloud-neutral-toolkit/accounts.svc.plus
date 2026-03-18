package service

import (
	"context"
	"errors"
	"slices"
	"strings"

	"account/internal/model"
	"account/internal/store"
	"gorm.io/gorm"
)

const homepageVideoDefaultDomainKey = "__default__"

type HomepageVideoEntry struct {
	DomainKey string
	VideoURL  string
	PosterURL string
}

type HomepageVideoSettings struct {
	DefaultEntry HomepageVideoEntry
	Overrides    []HomepageVideoEntry
}

func defaultHomepageVideoSettings() HomepageVideoSettings {
	return HomepageVideoSettings{
		DefaultEntry: HomepageVideoEntry{
			VideoURL: "https://www.youtube.com/watch?v=UW6DY6HQnmo",
		},
		Overrides: []HomepageVideoEntry{
			{
				DomainKey: "cn-www.svc.plus",
				VideoURL:  "https://www.bilibili.com/video/BV12DwazxEkL/?spm_id_from=333.1387.homepage.video_card.click&vd_source=e14d146f9a815c7d11e1a1fc23565ffd",
			},
		},
	}
}

func GetHomepageVideoSettings(ctx context.Context) (HomepageVideoSettings, error) {
	database := currentDB()
	if database == nil {
		return defaultHomepageVideoSettings(), nil
	}

	var rows []model.HomepageVideoSetting
	if err := database.WithContext(ctx).Order("domain_key ASC").Find(&rows).Error; err != nil {
		return HomepageVideoSettings{}, err
	}

	if len(rows) == 0 {
		return defaultHomepageVideoSettings(), nil
	}

	result := HomepageVideoSettings{}
	for _, row := range rows {
		entry := HomepageVideoEntry{
			DomainKey: normalizeHomepageVideoDomainKey(row.DomainKey),
			VideoURL:  strings.TrimSpace(row.VideoURL),
			PosterURL: strings.TrimSpace(row.PosterURL),
		}
		if row.DomainKey == homepageVideoDefaultDomainKey {
			result.DefaultEntry = entry
			continue
		}
		result.Overrides = append(result.Overrides, entry)
	}

	if strings.TrimSpace(result.DefaultEntry.VideoURL) == "" {
		result.DefaultEntry = defaultHomepageVideoSettings().DefaultEntry
	}
	slices.SortFunc(result.Overrides, func(left, right HomepageVideoEntry) int {
		return strings.Compare(left.DomainKey, right.DomainKey)
	})
	return result, nil
}

func SaveHomepageVideoSettings(
	ctx context.Context,
	settings HomepageVideoSettings,
	updatedBy string,
) (HomepageVideoSettings, error) {
	database := currentDB()
	if database == nil {
		return HomepageVideoSettings{}, ErrServiceDBNotInitialized
	}

	normalized, err := normalizeHomepageVideoSettings(settings)
	if err != nil {
		return HomepageVideoSettings{}, err
	}

	rows := make([]model.HomepageVideoSetting, 0, 1+len(normalized.Overrides))
	rows = append(rows, model.HomepageVideoSetting{
		DomainKey: homepageVideoDefaultDomainKey,
		VideoURL:  normalized.DefaultEntry.VideoURL,
		PosterURL: normalized.DefaultEntry.PosterURL,
		UpdatedBy: strings.TrimSpace(updatedBy),
	})

	for _, item := range normalized.Overrides {
		rows = append(rows, model.HomepageVideoSetting{
			DomainKey: item.DomainKey,
			VideoURL:  item.VideoURL,
			PosterURL: item.PosterURL,
			UpdatedBy: strings.TrimSpace(updatedBy),
		})
	}

	if err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.HomepageVideoSetting{}).Error; err != nil {
			return err
		}
		return tx.Create(&rows).Error
	}); err != nil {
		return HomepageVideoSettings{}, err
	}

	return normalized, nil
}

func ResolveHomepageVideoEntry(ctx context.Context, host string) (HomepageVideoEntry, error) {
	settings, err := GetHomepageVideoSettings(ctx)
	if err != nil {
		return HomepageVideoEntry{}, err
	}

	normalizedHost := store.NormalizeHostname(host)
	for _, item := range settings.Overrides {
		if item.DomainKey == normalizedHost {
			return item, nil
		}
	}
	return settings.DefaultEntry, nil
}

func normalizeHomepageVideoSettings(settings HomepageVideoSettings) (HomepageVideoSettings, error) {
	defaultVideoURL := strings.TrimSpace(settings.DefaultEntry.VideoURL)
	if defaultVideoURL == "" {
		return HomepageVideoSettings{}, errors.New("default videoUrl is required")
	}

	normalized := HomepageVideoSettings{
		DefaultEntry: HomepageVideoEntry{
			VideoURL:  defaultVideoURL,
			PosterURL: strings.TrimSpace(settings.DefaultEntry.PosterURL),
		},
	}

	seen := map[string]struct{}{}
	for _, item := range settings.Overrides {
		domainKey := normalizeHomepageVideoDomainKey(item.DomainKey)
		if domainKey == "" {
			return HomepageVideoSettings{}, errors.New("override domain is required")
		}
		if _, exists := seen[domainKey]; exists {
			return HomepageVideoSettings{}, errors.New("override domain must be unique")
		}
		videoURL := strings.TrimSpace(item.VideoURL)
		if videoURL == "" {
			return HomepageVideoSettings{}, errors.New("override videoUrl is required")
		}

		seen[domainKey] = struct{}{}
		normalized.Overrides = append(normalized.Overrides, HomepageVideoEntry{
			DomainKey: domainKey,
			VideoURL:  videoURL,
			PosterURL: strings.TrimSpace(item.PosterURL),
		})
	}

	slices.SortFunc(normalized.Overrides, func(left, right HomepageVideoEntry) int {
		return strings.Compare(left.DomainKey, right.DomainKey)
	})

	return normalized, nil
}

func normalizeHomepageVideoDomainKey(value string) string {
	normalized := store.NormalizeHostname(value)
	if normalized == homepageVideoDefaultDomainKey {
		return ""
	}
	return normalized
}
