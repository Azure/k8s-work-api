package work

import (
	"context"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
)

const (
	errFmtGetComponent = "cannot get component %q"
)

// ValidatingAppConfig is used for validating ApplicationConfiguration
type ValidatingWork struct {
	work workv1alpha1.Work
}

// PrepareForValidation prepares data for validations to avoiding repetitive GET/unmarshal operations
func (v *ValidatingWork) PrepareForValidation(ctx context.Context, c client.Reader, w *workv1alpha1.Work) error {
	v.work = *w
	retrievedWork := &workv1alpha1.Work{}
	err := wait.ExponentialBackoff(retry.DefaultBackoff, func() (bool, error) {
		var getErr error
		if getErr = c.Get(ctx, types.NamespacedName{Namespace: w.Namespace, Name: w.Name}, retrievedWork); getErr != nil {

		}

		if getErr != nil && !k8serrors.IsNotFound(getErr) {
			return false, getErr
		}

		return true, nil
	})

	if err != nil {
		return errors.Wrapf(err, errFmtGetComponent, w.Name)
	}

	return nil
}
