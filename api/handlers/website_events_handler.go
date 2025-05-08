package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/customeros/leads/internal/caches"
	"github.com/customeros/leads/internal/enum"
	"github.com/customeros/leads/internal/telemetry"
	"github.com/customeros/leads/internal/utils"
	"github.com/customeros/leads/proto/mappers"
	"github.com/customeros/leads/proto/pb"
	"github.com/customeros/leads/services"
)

type WebsiteEventsHandler struct {
	cache    *caches.OriginTenantCache
	services *services.Services
}

func NewWebsiteEventsHandler(services *services.Services) *WebsiteEventsHandler {
	return &WebsiteEventsHandler{
		cache:    caches.NewOriginTenantCache(),
		services: services,
	}
}

type WebTrackerEvent struct {
	IP               string               `json:"ip"`
	VisitorID        string               `json:"visitorId"`
	EventType        enum.WebTrackerEvent `json:"eventType"`
	EventData        string               `json:"eventData"`
	Timestamp        time.Time            `json:"timestamp"`
	Href             string               `json:"href"`
	Origin           string               `json:"origin"`
	Search           string               `json:"search"`
	Hostname         string               `json:"hostname"`
	Pathname         string               `json:"pathname"`
	Referrer         string               `json:"referrer"`
	UserAgent        string               `json:"userAgent"`
	Language         string               `json:"language"`
	CookiesEnabled   bool                 `json:"cookiesEnabled"`
	ScreenResolution string               `json:"screenResolution"`
}

const REQUEST_TIMEOUT = 60 * time.Second

func (h *WebsiteEventsHandler) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		span, ctx := telemetry.StartRestSpan(c.Request.Context(), "WebsiteEventsHandler.Handle")
		defer span.Finish()

		if err := h.validateHeaders(c, ctx); err != nil {
			span.TraceError(err)
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		tenant, webtrackerID, err := h.getTenantAndTrackerID(ctx, c.GetHeader("Origin"))
		if err != nil {
			span.TraceError(err)
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		ctx = utils.SetTenantInContext(ctx, tenant)

		trackerData := h.parsePayload(c, ctx)
		if trackerData == nil {
			err = fmt.Errorf("unable to build tracking record")
			span.TraceError(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"accepted": "true"})

		h.services.WebEventProcessor.Process(ctx, webtrackerID, &pb.WebTrackerEvent{
			VisitorId:        trackerData.VisitorID,
			Ip:               trackerData.IP,
			EventType:        proto_mappers.ConvertToProtoEventType(trackerData.EventType),
			EventData:        trackerData.EventData,
			Timestamp:        timestamppb.New(trackerData.Timestamp),
			Href:             trackerData.Href,
			Referrer:         trackerData.Referrer,
			UserAgent:        trackerData.UserAgent,
			Language:         trackerData.Language,
			CookiesEnabled:   trackerData.CookiesEnabled,
			ScreenResolution: trackerData.ScreenResolution,
		})
		return
	}
}

func (h *WebsiteEventsHandler) validateHeaders(c *gin.Context, ctx context.Context) error {
	span, ctx := telemetry.StartRestSpan(ctx, "WebsiteEventsHandler.validateHeaders")
	defer span.Finish()

	origin := c.GetHeader("Origin")
	referer := c.GetHeader("Referer")
	userAgent := c.GetHeader("User-Agent")

	span.LogKV("origin", origin)
	span.LogKV("referer", referer)
	span.LogKV("userAgent", userAgent)

	switch {
	case origin == "":
		err := errors.New("missing origin")
		span.LogFields(log.String("result.error", err.Error()))
		return err
	case referer == "":
		err := errors.New("missing referer")
		span.LogFields(log.String("result.error", err.Error()))
		return err
	case userAgent == "":
		err := errors.New("missing userAgent")
		span.LogFields(log.String("result.error", err.Error()))
		return err
	default:
		return nil
	}
}

func (h *WebsiteEventsHandler) getTenantAndTrackerID(ctx context.Context, origin string) (string, string, error) {
	span, ctx := telemetry.StartRestSpan(ctx, "WebsiteEventsHandler.getTenantAndTrackerID")
	defer span.Finish()
	span.LogKV("origin", origin)

	cleanedOrigin := utils.StripUrlToBasePath(origin)

	tenant, webtrackerID, err := h.cache.GetDataForOrigin(cleanedOrigin)
	if tenant != "" {
		span.LogKV("result.tenant.cached", tenant)
		span.LogKV("result.webtrackerID.cached", webtrackerID)
		return tenant, webtrackerID, nil
	}

	tenant, webtrackerID, err = h.findTrackerIDForOrigin(ctx, cleanedOrigin)
	if err != nil {
		span.TraceError(err)
		return "", "", err
	}

	span.LogKV("result.tenant", tenant)
	span.LogKV("result.webtrackerID", webtrackerID)
	return tenant, webtrackerID, err
}

func (h *WebsiteEventsHandler) findTrackerIDForOrigin(ctx context.Context, cleanedOrigin string) (string, string, error) {
	span, ctx := telemetry.StartRestSpan(ctx, "WebsiteEventsHandler.findTrackerIDForOrigin")
	defer span.Finish()
	span.LogKV("cleanedOrigin", cleanedOrigin)

	webtracker, err := h.services.WebtrackerService.GetWebtrackerByOrigin(ctx, cleanedOrigin)
	if err != nil {
		span.TraceError(err)
		return "", "", err
	}
	if webtracker == nil {
		err = fmt.Errorf("webtracker not found for origin: %s", cleanedOrigin)
		return "", "", err
	}

	err = h.cache.SetDataForOrigin(cleanedOrigin, webtracker.Tenant, webtracker.ID)
	if err != nil {
		span.TraceError(err)
	}

	return webtracker.Tenant, webtracker.ID, nil
}

func (h *WebsiteEventsHandler) parsePayload(c *gin.Context, ctx context.Context) *WebTrackerEvent {
	span, _ := telemetry.StartRestSpan(ctx, "WebsiteEventsHandler.parsePayload")
	defer span.Finish()

	tracking := WebTrackerEvent{}

	rawJSON, err := c.GetRawData()
	if err != nil {
		span.TraceError(err)
		return nil
	}
	span.LogFields(log.String("rawJSON", string(rawJSON)))

	var inputMap map[string]any
	if err = json.Unmarshal(rawJSON, &inputMap); err != nil {
		span.TraceError(err)
		return nil
	}

	if err = utils.Decode(inputMap, &tracking); err != nil {
		span.TraceError(err)
		return nil
	}
	tracking.UserAgent = utils.SanitizeUTF8(tracking.UserAgent)
	tracking.Referrer = utils.SanitizeUTF8(tracking.Referrer)
	tracking.Href = utils.SanitizeUTF8(tracking.Href)
	tracking.VisitorID = utils.SanitizeUTF8(tracking.VisitorID)
	tracking.Origin = utils.SanitizeUTF8(tracking.Origin)
	tracking.Search = utils.SanitizeUTF8(tracking.Search)
	tracking.Hostname = utils.SanitizeUTF8(tracking.Hostname)
	tracking.Pathname = utils.SanitizeUTF8(tracking.Pathname)

	return &tracking
}
