package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/avnpl/go-march/services"
	"github.com/avnpl/go-march/utils"
	"github.com/avnpl/go-march/utils/customErrors"
	"github.com/avnpl/go-march/utils/log"
	"go.uber.org/zap"
)

type AnalyticsHandler struct {
	svc    services.AnalyticsService
	logger *zap.Logger
}

func NewAnalyticsHandler(svc services.AnalyticsService, logger *zap.Logger) AnalyticsHandler {
	return AnalyticsHandler{svc: svc, logger: logger}
}

func (h AnalyticsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/analytics/sales", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			SendErrorResponse(r.Context(), w, customErrors.InvalidHTTPMethod)
			return
		}
		h.getTotalSales(w, r)
	})
	mux.HandleFunc("/analytics/average-order-value", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			SendErrorResponse(r.Context(), w, customErrors.InvalidHTTPMethod)
			return
		}
		h.getAverageOrderValue(w, r)
	})
	mux.HandleFunc("/analytics/top-products", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			SendErrorResponse(r.Context(), w, customErrors.InvalidHTTPMethod)
			return
		}
		h.getTopProducts(w, r)
	})
	mux.HandleFunc("/analytics/low-stock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			SendErrorResponse(r.Context(), w, customErrors.InvalidHTTPMethod)
			return
		}
		h.getLowStockProducts(w, r)
	})
}

func (h AnalyticsHandler) getTotalSales(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	totalOrders, totalRevenue, err := h.svc.GetTotalSales(ctx, start, end)
	if err != nil {
		log.Error(ctx, h.logger, "GetTotalSales failed", zap.Error(err))
		SendErrorResponse(ctx, w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_orders":  totalOrders,
		"total_revenue": totalRevenue,
	})
}

func (h AnalyticsHandler) getAverageOrderValue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	start, end, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	avg, err := h.svc.GetAverageOrderValue(ctx, start, end)
	if err != nil {
		log.Error(ctx, h.logger, "GetAverageOrderValue failed", zap.Error(err))
		SendErrorResponse(ctx, w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"average_order_value": avg,
	})
}

func (h AnalyticsHandler) getTopProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, ok := parseNonNegativeQueryInt(w, r, "limit", 10)
	if !ok {
		return
	}
	if limit == 0 {
		limit = 10
	}

	stats, err := h.svc.GetTopProducts(ctx, limit)
	if err != nil {
		log.Error(ctx, h.logger, "GetTopProducts failed", zap.Error(err))
		SendErrorResponse(ctx, w, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h AnalyticsHandler) getLowStockProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	threshold, ok := parseNonNegativeQueryInt(w, r, "threshold", 0)
	if !ok {
		return
	}

	products, err := h.svc.GetLowStockProducts(ctx, threshold)
	if err != nil {
		log.Error(ctx, h.logger, "GetLowStockProducts failed", zap.Error(err))
		SendErrorResponse(ctx, w, err)
		return
	}

	writeJSON(w, http.StatusOK, products)
}

func parseDateRange(w http.ResponseWriter, r *http.Request) (time.Time, time.Time, bool) {
	start, ok := parseRFC3339Query(w, r, "start_date")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	end, ok := parseRFC3339Query(w, r, "end_date")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func parseRFC3339Query(w http.ResponseWriter, r *http.Request, name string) (time.Time, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		utils.SendJSONError(w, http.StatusBadRequest, name+" is required")
		return time.Time{}, false
	}

	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		utils.SendJSONError(w, http.StatusBadRequest, "invalid "+name+", expected RFC3339")
		return time.Time{}, false
	}
	return t, true
}

func parseNonNegativeQueryInt(w http.ResponseWriter, r *http.Request, name string, defaultVal int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultVal, true
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		utils.SendJSONError(w, http.StatusBadRequest, "invalid query param, "+name)
		return 0, false
	}
	if n < 0 {
		utils.SendJSONError(w, http.StatusBadRequest, name+" must be non-negative")
		return 0, false
	}
	return n, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
