// Hand-generated DeepCopy implementations for Remediation API types (github-app copy).
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies the receiver into out.
func (in *Remediation) DeepCopyInto(out *Remediation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopyObject returns a copy for runtime.Object.
func (in *Remediation) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a copy of the receiver.
func (in *Remediation) DeepCopy() *Remediation {
	if in == nil {
		return nil
	}
	out := new(Remediation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *RemediationList) DeepCopyInto(out *RemediationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Remediation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyObject returns a copy for runtime.Object.
func (in *RemediationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy returns a copy of the receiver.
func (in *RemediationList) DeepCopy() *RemediationList {
	if in == nil {
		return nil
	}
	out := new(RemediationList)
	in.DeepCopyInto(out)
	return out
}

func (in *RemediationSpec) DeepCopyInto(out *RemediationSpec) {
	*out = *in
	in.Alert.DeepCopyInto(&out.Alert)
	in.Target.DeepCopyInto(&out.Target)
	in.Action.DeepCopyInto(&out.Action)
	in.Strategy.DeepCopyInto(&out.Strategy)
	if in.GitHub != nil {
		in, out := &in.GitHub, &out.GitHub
		*out = new(GitHubConfig)
		(*in).DeepCopyInto(*out)
	}
}

func (in *AlertInfo) DeepCopyInto(out *AlertInfo) { *out = *in }

func (in *TargetResource) DeepCopyInto(out *TargetResource) { *out = *in }

func (in *Action) DeepCopyInto(out *Action) {
	*out = *in
	if in.Params != nil {
		in, out := &in.Params, &out.Params
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *Strategy) DeepCopyInto(out *Strategy) { *out = *in }

func (in *GitHubConfig) DeepCopyInto(out *GitHubConfig) {
	*out = *in
	if in.PRLabels != nil {
		in, out := &in.PRLabels, &out.PRLabels
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *RemediationStatus) DeepCopyInto(out *RemediationStatus) {
	*out = *in
	if in.AppliedAt != nil {
		in, out := &in.AppliedAt, &out.AppliedAt
		*out = (*in).DeepCopy()
	}
	if in.ResolvedAt != nil {
		in, out := &in.ResolvedAt, &out.ResolvedAt
		*out = (*in).DeepCopy()
	}
	if in.LastUpdateTime != nil {
		in, out := &in.LastUpdateTime, &out.LastUpdateTime
		*out = (*in).DeepCopy()
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}
