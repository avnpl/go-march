package rest

import (
	"context"
	"net/http"

	"github.com/avnpl/go-march/utils"
	"github.com/avnpl/go-march/utils/customErrors"
)

func SendErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
	if status, ok := customErrors.HTTPFor(err); ok {
		utils.SendJSONError(w, status, err.Error())
	} else {
		utils.SendInternalError(w)
	}
}
