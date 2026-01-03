package sts

import (
	"cloudlocal/internal/utils"
	"fmt"
	"net/http"
	"net/url"
)

var CloudlocalAdmin = fmt.Sprintf("arn:aws:iam::%s:user/cloudlocal-admin", utils.AccountId)

type ServiceHandler interface {
	Handle(w http.ResponseWriter, body url.Values)
}

func NewStsService() ServiceHandler {
	var stsEnabled = utils.IsServiceEnabled("sts")

	if stsEnabled {
		return &stsServiceImplementation{
			sts: newSts(),
		}
	}
	return nil
}

func (svc *stsServiceImplementation) Handle(w http.ResponseWriter, body url.Values) {
	action := body.Get("Action")
	switch action {
	case "GetCallerIdentity":
		result := svc.sts.getCallerIdentity()
		utils.RespondJSON(w, result)
		w.WriteHeader(http.StatusOK)
	}
}

type internalSts interface {
	getCallerIdentity() GetCallerIdentityResponse
}

func newSts() internalSts {
	return &stsImplementation{}
}

func (s *stsImplementation) getCallerIdentity() GetCallerIdentityResponse {
	return GetCallerIdentityResponse{
		Arn:     CloudlocalAdmin,
		Account: utils.AccountId,
		UserId:  "AIDACKCEVSQ6C2EXAMPLE",
	}
}
