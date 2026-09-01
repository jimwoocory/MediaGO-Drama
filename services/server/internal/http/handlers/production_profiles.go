package handlers

import (
	"github.com/gin-gonic/gin"
	httpresponse "github.com/mediago-dev/mediago-drama/services/server/internal/http/response"
	serviceproductionprofile "github.com/mediago-dev/mediago-drama/services/server/internal/service/productionprofile"
)

// ProductionProfiles handles read-only built-in production planning profiles.
type ProductionProfiles struct {
	registry *serviceproductionprofile.Registry
}

func NewProductionProfiles(registry *serviceproductionprofile.Registry) ProductionProfiles {
	return ProductionProfiles{registry: registry}
}

type productionProfileListResponse struct {
	SchemaVersion int                                `json:"schemaVersion"`
	Profiles      []serviceproductionprofile.Profile `json:"profiles"`
}

func (handler ProductionProfiles) HandleListProductionProfiles(context *gin.Context) {
	profiles := []serviceproductionprofile.Profile{}
	if handler.registry != nil {
		profiles = handler.registry.List()
	}
	httpresponse.OK(context, productionProfileListResponse{
		SchemaVersion: serviceproductionprofile.ManifestSchemaVersion,
		Profiles:      profiles,
	})
}
