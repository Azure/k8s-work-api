package work

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"net/http/httptest"
	workapi "sigs.k8s.io/work-api/pkg/apis/v1alpha1"
	"testing"
)

func TestHappyPath(t *testing.T) {
	work := &workapi.Work{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: workapi.WorkSpec{
			Workload: workapi.WorkloadTemplate{
				Manifests: nil,
			},
		},
	}

	workJSON, _ := json.Marshal(work)
	admReview := v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: workJSON,
			},
		},
	}

	admReviewJSON, _ := json.Marshal(admReview)

	r := httptest.NewRequest(http.MethodPost, "/todo", bytes.NewReader(admReviewJSON))
	w := httptest.NewRecorder()

	validate(w, r)
	response := w.Result()

	responseBytes, _ := ioutil.ReadAll(response.Body)
	receivedAdmReview := v1beta1.AdmissionReview{}
	json.Unmarshal(responseBytes, &receivedAdmReview)

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, true, receivedAdmReview.Response.Allowed)
}

func validate(w http.ResponseWriter, r *http.Request) {
	// Todo
}
