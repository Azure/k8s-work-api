package work

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

const (
	validWorkRequest = "Valid Work request."
)

// +kubebuilder:webhook:path=/validate-v1-pod,mutating=false,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=vpod.kb.io

// Validates Works
type workValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// Handle the validation of Work resources.
func (v *workValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	work := &workv1alpha1.Work{}

	err := v.decoder.Decode(req, work)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// ToDo - Validation of Work object
	return admission.Allowed(validWorkRequest)

}

func (v *workValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
