package work

import (
	"context"
	"k8s.io/apimachinery/pkg/util/validation/field"

	admissionv1 "k8s.io/api/admission/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

const (
	validWorkRequest = "Valid Work request."
)

type WorkValidator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// Handle the validation of Work resources.
func (v *WorkValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	work := &workv1alpha1.Work{}
	err := v.Decoder.Decode(req, work)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = v.Decoder.Decode(req, work)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Delete:
		// Todo -  Validate delete logic
	case admissionv1.Update:
		// Todo -  Validate create logic
	case admissionv1.Create:
	default:
	}

	return admission.ValidationResponse(true, "")
}

// ValidateCreate validates the Work on creation.
func (v *WorkValidator) ValidateCreate(ctx context.Context, obj *workv1alpha1.Work) field.ErrorList {
	// Todo
	return nil
}

// ValidateUpdate validates the Work on update.
func (v *WorkValidator) ValidateUpdate(ctx context.Context, newWork, oldWork *workv1alpha1.Work) field.ErrorList {
	// Todo
	return nil
}

// InjectClient injects the client into the WorkValidator.
func (v *WorkValidator) InjectClient(c client.Client) error {
	v.Client = c
	return nil
}

// InjectDecoder injects an admission.Decoder into the WorkValidator.
func (v *WorkValidator) InjectDecoder(d *admission.Decoder) error {
	v.Decoder = d
	return nil
}
