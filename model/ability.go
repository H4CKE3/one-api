package model

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/utils"
)

var ErrNoChannelAvailableWithinCooldown = errors.New("all candidate channels are cooling down after rate limit")

type Ability struct {
	Group     string `json:"group" gorm:"type:varchar(32);primaryKey;autoIncrement:false"`
	Model     string `json:"model" gorm:"primaryKey;autoIncrement:false"`
	ChannelId int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool   `json:"enabled"`
	Priority  *int64 `json:"priority" gorm:"bigint;default:0;index"`
}

type channelCandidate struct {
	channelID int
	priority  int64
}

func selectChannelID(candidates []channelCandidate, ignoreFirstPriority bool, isCooling func(channelID int) bool, randomIntn func(n int) int) (int, error) {
	if len(candidates) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	if randomIntn == nil {
		randomIntn = rand.Intn
	}
	startIdx := 0
	firstPriorityEnd := len(candidates)
	firstPriority := candidates[0].priority
	for i := range candidates {
		if candidates[i].priority != firstPriority {
			firstPriorityEnd = i
			break
		}
	}
	if ignoreFirstPriority && firstPriorityEnd < len(candidates) {
		startIdx = firstPriorityEnd
	}
	for poolStart := startIdx; poolStart < len(candidates); {
		poolEnd := poolStart + 1
		for poolEnd < len(candidates) && candidates[poolEnd].priority == candidates[poolStart].priority {
			poolEnd++
		}
		available := make([]int, 0, poolEnd-poolStart)
		for i := poolStart; i < poolEnd; i++ {
			if !isCooling(candidates[i].channelID) {
				available = append(available, candidates[i].channelID)
			}
		}
		if len(available) > 0 {
			return available[randomIntn(len(available))], nil
		}
		poolStart = poolEnd
	}
	if config.ChannelRateLimitCooldownSeconds > 0 {
		return 0, ErrNoChannelAvailableWithinCooldown
	}
	return 0, gorm.ErrRecordNotFound
}

func GetRandomSatisfiedChannel(group string, model string, ignoreFirstPriority bool) (*Channel, error) {
	groupCol := "`group`"
	trueVal := "1"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
		trueVal = "true"
	}

	channelQuery := DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, model)
	var abilities []Ability
	err := channelQuery.Order("priority desc, channel_id asc").Find(&abilities).Error
	if err != nil {
		return nil, err
	}
	candidates := make([]channelCandidate, 0, len(abilities))
	for _, ability := range abilities {
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		candidates = append(candidates, channelCandidate{
			channelID: ability.ChannelId,
			priority:  priority,
		})
	}
	channelID, err := selectChannelID(candidates, ignoreFirstPriority, IsChannelRateLimited, rand.Intn)
	if err != nil {
		return nil, err
	}
	channel := Channel{}
	channel.Id = channelID
	err = DB.First(&channel, "id = ?", channelID).Error
	return &channel, err
}

func (channel *Channel) AddAbilities() error {
	models_ := strings.Split(channel.Models, ",")
	models_ = utils.DeDuplication(models_)
	groups_ := strings.Split(channel.Group, ",")
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == ChannelStatusEnabled,
				Priority:  channel.Priority,
			}
			abilities = append(abilities, ability)
		}
	}
	return DB.Create(&abilities).Error
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities() error {
	err := channel.DeleteAbilities()
	if err != nil {
		return err
	}
	err = channel.AddAbilities()
	if err != nil {
		return err
	}
	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func GetGroupModels(ctx context.Context, group string) ([]string, error) {
	groupCol := "`group`"
	trueVal := "1"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
		trueVal = "true"
	}
	var models []string
	err := DB.Model(&Ability{}).Distinct("model").Where(groupCol+" = ? and enabled = "+trueVal, group).Pluck("model", &models).Error
	if err != nil {
		return nil, err
	}
	sort.Strings(models)
	return models, err
}
