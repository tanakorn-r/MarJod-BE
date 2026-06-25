package controller

import (
	"errors"
	"finance-chat/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type QuestController struct {
	svc service.QuestService
}

func NewQuestController(svc service.QuestService) *QuestController {
	return &QuestController{svc: svc}
}

// GetQuests godoc
// @Summary      Get the user's current quest board
// @Description  Returns the user's current quest batch with progress evaluated live from real transactions, plus game profile (level/XP) and streak. Quests are auto-completed and XP is awarded idempotently as soon as their progress condition is met.
// @Tags         quests
// @Produce      json
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

// RerollQuests godoc
// @Summary      Request a new set of quests
// @Description  Picks a new random batch of quest templates. Rejected with 400 if the current batch still has an incomplete quest.
// @Tags         quests
// @Produce      json
// @Success      200  {object}  service.QuestBoard
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/quests/reroll [post]
func (c *QuestController) RerollQuests(ctx *gin.Context) {
	board, err := c.svc.RerollQuests(requestUserID(ctx))
	if err != nil {
		if errors.Is(err, service.ErrQuestsInProgress) {
			ctx.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		ctx.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, board)
}
