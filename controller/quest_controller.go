package controller

import (
	"errors"
	"finance-chat/model"
	"finance-chat/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type QuestController struct {
	svc service.QuestService
}

func NewQuestController(svc service.QuestService) *QuestController {
	return &QuestController{svc: svc}
}

type CreateQuestPresetRequest struct {
	Key         model.QuestTemplateKey `json:"key" binding:"required" example:"coffee_cap_weekly_250"`
	Name        string                 `json:"name" binding:"required" example:"Coffee discipline"`
	Logo        string                 `json:"logo" example:"☕"`
	Period      model.QuestPeriod      `json:"period" binding:"required" example:"weekly"`
	Difficulty  model.QuestDifficulty  `json:"difficulty" binding:"required" example:"advanced"`
	XP          int                    `json:"xp" binding:"required" example:"50"`
	Accent      string                 `json:"accent" example:"#D9463B"`
	RuleType    model.QuestRuleType    `json:"rule_type" binding:"required" example:"coffee_spend_cap"`
	Target      float64                `json:"target" example:"500"`
	Unit        string                 `json:"unit" example:"thb"`
	Category    string                 `json:"category" example:"Food & Beverage"`
	SubCategory string                 `json:"sub_category" example:"Coffee"`
	BehaviorTag string                 `json:"behavior_tag" example:"impulse"`
	IsActive    *bool                  `json:"is_active"`
}

type UpdateQuestPresetRequest struct {
	Key         *model.QuestTemplateKey `json:"key"`
	Name        *string                 `json:"name"`
	Logo        *string                 `json:"logo"`
	Period      *model.QuestPeriod      `json:"period"`
	Difficulty  *model.QuestDifficulty  `json:"difficulty"`
	XP          *int                    `json:"xp"`
	Accent      *string                 `json:"accent"`
	RuleType    *model.QuestRuleType    `json:"rule_type"`
	Target      *float64                `json:"target"`
	Unit        *string                 `json:"unit"`
	Category    *string                 `json:"category"`
	SubCategory *string                 `json:"sub_category"`
	BehaviorTag *string                 `json:"behavior_tag"`
	IsActive    *bool                   `json:"is_active"`
}

type GenerateQuestsRequest struct {
	Period model.QuestPeriod `json:"period" binding:"required" example:"daily"`
}

// GetQuests godoc
// @Summary      Get the user's current quest board
// @Description  Returns the user's active quest batches with progress evaluated live from real transactions, plus game profile (level/XP) and streak. This read does not auto-assign quests; call POST /api/quests/generate to activate daily or weekly quests.
// @Tags         quests
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  service.QuestBoard
// @Failure      500  {object}  ErrorResponse
// @Router       /api/quests [get]
func (c *QuestController) GetQuests(ctx *gin.Context) {
	board, err := c.svc.GetQuests(requestUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, board)
}

// GenerateQuests godoc
// @Summary      Generate and activate daily or weekly quests
// @Description  Randomly picks active preset quests for the requested period if the user does not already have an active batch. Daily quests expire at next midnight; weekly quests expire after Sunday at next Monday 00:00.
// @Tags         quests
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      GenerateQuestsRequest  true  "Quest period to activate"
// @Success      200   {object}  service.QuestBoard
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/quests/generate [post]
func (c *QuestController) GenerateQuests(ctx *gin.Context) {
	var req GenerateQuestsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	board, err := c.svc.GenerateQuests(requestUserID(ctx), req.Period)
	if err != nil {
		if errors.Is(err, service.ErrInvalidQuestPeriod) || errors.Is(err, service.ErrNoActiveQuestPresets) || errors.Is(err, service.ErrQuestSetAlreadyGenerated) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, board)
}

// RerollQuests godoc
// @Summary      Request a new set of quests
// @Description  Picks a new random batch of quest templates. Rejected with 400 if the current batch still has an incomplete quest.
// @Tags         quests
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  service.QuestBoard
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/quests/reroll [post]
func (c *QuestController) RerollQuests(ctx *gin.Context) {
	board, err := c.svc.RerollQuests(requestUserID(ctx))
	if err != nil {
		if errors.Is(err, service.ErrQuestsInProgress) || errors.Is(err, service.ErrNoActiveQuestPresets) || errors.Is(err, service.ErrQuestSetAlreadyGenerated) || errors.Is(err, service.ErrQuestRerollUnavailable) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, board)
}

// ListQuestPresets godoc
// @Summary      List quest presets
// @Description  Returns DB-backed preset quests used by the reward assignment pool. Pass include_inactive=true to include archived presets.
// @Tags         quests
// @Produce      json
// @Security     BearerAuth
// @Param        include_inactive  query  bool  false  "Include inactive/deleted presets"
// @Success      200  {array}   model.QuestPreset
// @Failure      500  {object}  ErrorResponse
// @Router       /api/quest-presets [get]
func (c *QuestController) ListQuestPresets(ctx *gin.Context) {
	presets, err := c.svc.ListQuestPresets(ctx.Query("include_inactive") == "true")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, presets)
}

// CreateQuestPreset godoc
// @Summary      Create a quest preset
// @Description  Creates a daily or weekly preset quest definition. User assignment/completion state remains stored separately.
// @Tags         quests
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      CreateQuestPresetRequest  true  "Quest preset"
// @Success      201   {object}  model.QuestPreset
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/quest-presets [post]
func (c *QuestController) CreateQuestPreset(ctx *gin.Context) {
	var req CreateQuestPresetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	preset := &model.QuestPreset{
		Key:         req.Key,
		Name:        req.Name,
		Logo:        req.Logo,
		Period:      req.Period,
		Difficulty:  req.Difficulty,
		XP:          req.XP,
		Accent:      req.Accent,
		RuleType:    req.RuleType,
		Target:      req.Target,
		Unit:        req.Unit,
		Category:    req.Category,
		SubCategory: req.SubCategory,
		BehaviorTag: req.BehaviorTag,
		IsActive:    isActive,
	}
	created, err := c.svc.CreateQuestPreset(preset)
	if err != nil {
		if errors.Is(err, service.ErrInvalidQuestPreset) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, created)
}

// UpdateQuestPreset godoc
// @Summary      Update a quest preset
// @Description  Updates editable preset quest metadata/rules. Existing user assignments keep their stored template key and history.
// @Tags         quests
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int                       true  "Quest preset ID"
// @Param        body  body      UpdateQuestPresetRequest  true  "Quest preset fields"
// @Success      200   {object}  model.QuestPreset
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /api/quest-presets/{id} [patch]
func (c *QuestController) UpdateQuestPreset(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return
	}
	var req UpdateQuestPresetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	updated, err := c.svc.UpdateQuestPreset(uint(id), service.QuestPresetPatch(req))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, ErrorResponse{Error: "quest preset not found"})
			return
		}
		if errors.Is(err, service.ErrInvalidQuestPreset) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, updated)
}

// DeleteQuestPreset godoc
// @Summary      Delete a quest preset
// @Description  Archives a preset by setting is_active=false so existing user assignment history remains readable.
// @Tags         quests
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Quest preset ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/quest-presets/{id} [delete]
func (c *QuestController) DeleteQuestPreset(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return
	}
	if err := c.svc.DeleteQuestPreset(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, ErrorResponse{Error: "quest preset not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "quest preset archived"})
}
