package vpclattice

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/vpclattice"
	"github.com/aws/aws-sdk-go-v2/service/vpclattice/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_vpclattice_listener_rule", name="Listener Rule")
// @Tags(identifierAttribute="arn")
func ResourceListenerRule() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceListenerRuleCreate,
		ReadWithoutTimeout:   resourceListenerRuleRead,
		UpdateWithoutTimeout: resourceListenerRuleUpdate,
		DeleteWithoutTimeout: resourceListenerRuleDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"action": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"fixed_response": {
							Type:     schema.TypeList,
							MaxItems: 1,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"status_code": {
										Type:         schema.TypeInt,
										Required:     true,
										ValidateFunc: validation.IntBetween(100, 599),
									},
								},
							},
							DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
						},
						"forward": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"target_groups": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										MaxItems: 2,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"target_group_identifier": {
													Type:     schema.TypeString,
													Required: true,
												},
												"weight": {
													Type:         schema.TypeInt,
													ValidateFunc: validation.IntBetween(0, 999),
													Default:      1,
													Optional:     true,
												},
											},
										},
									},
								},
							},
							DiffSuppressFunc: verify.SuppressMissingOptionalConfigurationBlock,
						},
					},
				},
			},
			"match": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"http_match": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"method": {
										Type:     schema.TypeString,
										Computed: true,
										Optional: true,
									},
									"headers_matches": {
										Type:     schema.TypeList,
										Optional: true,
										MinItems: 1,
										MaxItems: 5,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"case_sensitive": {
													Type:     schema.TypeBool,
													Optional: true,
												},
												"match": {
													Type:     schema.TypeList,
													Optional: true,
													MaxItems: 1,
													Elem: &schema.Resource{
														Schema: map[string]*schema.Schema{
															"contains": {
																Type:     schema.TypeString,
																Optional: true,
															},
															"exact": {
																Type:     schema.TypeString,
																Optional: true,
															},
															"prefix": {
																Type:     schema.TypeString,
																Optional: true,
															},
														},
													},
												},
												"name": {
													Type:     schema.TypeString,
													Optional: true,
												},
											},
										},
									},
									"path_match": {
										Type:     schema.TypeList,
										Optional: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"case_sensitive": {
													Type:     schema.TypeBool,
													Optional: true,
												},
												"match": {
													Type:     schema.TypeList,
													Optional: true,
													MaxItems: 1,
													Elem: &schema.Resource{
														Schema: map[string]*schema.Schema{
															"exact": {
																Type:     schema.TypeString,
																Optional: true,
															},
															"prefix": {
																Type:     schema.TypeString,
																Optional: true,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(3, 128),
			},
			"priority": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ForceNew: false,
			},

			"listener_identifier": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"service_identifier": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
		},

		CustomizeDiff: customdiff.All(
			verify.SetTagsDiff,
		),
	}
}

const (
	ResNameListenerRule = "Listener Rule"
)

func resourceListenerRuleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).VPCLatticeClient()

	name := d.Get("name").(string)
	in := &vpclattice.CreateRuleInput{
		ClientToken:        aws.String(id.UniqueId()),
		Name:               aws.String(name),
		ListenerIdentifier: aws.String(d.Get("listener_identifier").(string)),
		ServiceIdentifier:  aws.String(d.Get("service_identifier").(string)),
		Tags:               GetTagsIn(ctx),
	}
	if v, ok := d.GetOk("action"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
		in.Action = expandRuleAction(v.([]interface{})[0].(map[string]interface{}))
	}

	if v, ok := d.GetOk("match"); ok && len(v.([]interface{})) > 0 && v.([]interface{})[0] != nil {
		in.Match = expandRuleMatch(v.([]interface{})[0].(map[string]interface{}))
	}

	out, err := conn.CreateRule(ctx, in)

	if err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionCreating, ResNameService, name, err)
	}

	d.SetId(aws.ToString(out.Id)) //Concatinate my ids to one

	if _, err := waitTargetGroupCreated(ctx, conn, d.Id(), d.Timeout(schema.TimeoutCreate)); err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionWaitingForCreation, ResNameTargetGroup, d.Id(), err)
	}

	return resourceTargetGroupRead(ctx, d, meta)
}

func resourceListenerRuleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).VPCLatticeClient()
	//split the concatinate ids

	out, err := FindListenerRuleByID(ctx, conn, d.Id(), d.Get("listener_identifier").(string), d.Get("service_identifier").(string))

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] VpcLattice Listener Rule (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionReading, ResNameListenerRule, d.Id(), err)
	}

	d.Set("arn", out.Arn)

	if err := d.Set("action", []interface{}{flattenRuleAction(out.Action)}); err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionSetting, ResNameListenerRule, d.Id(), err)
	}

	if err := d.Set("match", []interface{}{flattenRuleMatch(out.Match)}); err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionSetting, ResNameListenerRule, d.Id(), err)
	}

	d.Set("name", out.Name)

	return nil
}

func resourceListenerRuleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).VPCLatticeClient()

	if d.HasChangesExcept("tags", "tags_all") {
		in := &vpclattice.BatchUpdateRuleInput{
			// Rules:              []aws.String(d.Id()),
			ListenerIdentifier: aws.String(d.Get("listener_identifier").(string)),
			ServiceIdentifier:  aws.String(d.Get("service_identifier").(string)),
		}

		// out, err :=
		conn.BatchUpdateRule(ctx, in)

		// if err != nil {
		// 	return create.DiagError(names.VPCLattice, create.ErrActionUpdating, ResNameTargetGroup, d.Id(), err)
		// }

		// if _, err := waitTargetGroupUpdated(ctx, conn, aws.ToString(out.Id), d.Timeout(schema.TimeoutUpdate)); err != nil {
		// 	return create.DiagError(names.VPCLattice, create.ErrActionWaitingForUpdate, ResNameTargetGroup, d.Id(), err)
		// }
	}

	return resourceTargetGroupRead(ctx, d, meta)
}
func resourceListenerRuleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).VPCLatticeClient()

	log.Printf("[INFO] Deleting VpcLattice ListeningRule: %s", d.Id())
	_, err := conn.DeleteRule(ctx, &vpclattice.DeleteRuleInput{
		RuleIdentifier:     aws.String(d.Id()),
		ListenerIdentifier: aws.String(d.Get("listener_identifier").(string)),
		ServiceIdentifier:  aws.String(d.Get("service_identifier").(string)),
	})

	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil
		}

		return create.DiagError(names.VPCLattice, create.ErrActionDeleting, ResNameTargetGroup, d.Id(), err)
	}

	if _, err := waitTargetGroupDeleted(ctx, conn, d.Id(), d.Timeout(schema.TimeoutDelete)); err != nil {
		return create.DiagError(names.VPCLattice, create.ErrActionWaitingForDeletion, ResNameTargetGroup, d.Id(), err)
	}

	return nil
}

// const (
// 	statusChangePending = "Pending"
// 	statusDeleting      = "Deleting"
// 	statusNormal        = "Normal"
// 	statusUpdated       = "Updated"
// )

// func waitListenerRuleCreated(ctx context.Context, conn *vpclattice.Client, id string, timeout time.Duration) (*vpclattice.ListenerRule, error) {
// 	stateConf := &resource.StateChangeConf{
// 		Pending:                   []string{},
// 		Target:                    []string{statusNormal},
// 		Refresh:                   statusListenerRule(ctx, conn, id),
// 		Timeout:                   timeout,
// 		NotFoundChecks:            20,
// 		ContinuousTargetOccurence: 2,
// 	}

// 	outputRaw, err := stateConf.WaitForStateContext(ctx)
// 	if out, ok := outputRaw.(*vpclattice.ListenerRule); ok {
// 		return out, err
// 	}

// 	return nil, err
// }

// func waitListenerRuleUpdated(ctx context.Context, conn *vpclattice.Client, id string, timeout time.Duration) (*vpclattice.GetRuleOutput, error) {
// 	stateConf := &resource.StateChangeConf{
// 		Pending:                   []string{statusChangePending},
// 		Target:                    []string{statusUpdated},
// 		Refresh:                   statusListenerRule(ctx, conn, id),
// 		Timeout:                   timeout,
// 		NotFoundChecks:            20,
// 		ContinuousTargetOccurence: 2,
// 	}

// 	outputRaw, err := stateConf.WaitForStateContext(ctx)
// 	if out, ok := outputRaw.(*vpclattice.ListenerRule); ok {
// 		return out, err
// 	}

// 	return nil, err
// }

// func waitListenerRuleDeleted(ctx context.Context, conn *vpclattice.Client, id, listenerIdentifier, serviceIdentifier string, timeout time.Duration) (*vpclattice.GetRuleOutput, error) {
// 	stateConf := &resource.StateChangeConf{
// 		Pending: []string{statusDeleting, statusNormal},
// 		Target:  []string{},
// 		Refresh: statusListenerRule(ctx, conn, id, listenerIdentifier, serviceIdentifier),
// 		Timeout: timeout,
// 	}

// 	outputRaw, err := stateConf.WaitForStateContext(ctx)
// 	if out, ok := outputRaw.(*vpclattice.ListenerRule); ok {
// 		return out, err
// 	}

// 	return nil, err
// }

// func statusListenerRule(ctx context.Context, conn *vpclattice.Client, id, listenerIdentifier, serviceIdentifier string) resource.StateRefreshFunc {
// 	return func() (interface{}, string, error) {
// 		out, err := FindListenerRuleByID(ctx, conn, id, listenerIdentifier, serviceIdentifier)
// 		if tfresource.NotFound(err) {
// 			return nil, "", nil
// 		}

// 		if err != nil {
// 			return nil, "", err
// 		}

// 		return out, aws.ToString(out.), nil
// 	}
// }

func FindListenerRuleByID(ctx context.Context, conn *vpclattice.Client, id, listenerIdentifier, serviceIdentifier string) (*vpclattice.GetRuleOutput, error) {
	in := &vpclattice.GetRuleInput{
		RuleIdentifier:     aws.String(id),
		ListenerIdentifier: aws.String(listenerIdentifier),
		ServiceIdentifier:  aws.String(serviceIdentifier),
	}
	out, err := conn.GetRule(ctx, in)
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil, &retry.NotFoundError{
				LastError:   err,
				LastRequest: in,
			}
		}

		return nil, err
	}
	if out == nil || out.Id == nil {
		return nil, tfresource.NewEmptyResultError(in)
	}

	return out, nil
}

func flattenRuleAction(apiObject types.RuleAction) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := make(map[string]interface{})

	if v, ok := apiObject.(*types.RuleActionMemberFixedResponse); ok {
		tfMap["fixed_response"] = flattenRuleActionMemberFixedResponse(v)
	}
	if v, ok := apiObject.(*types.RuleActionMemberForward); ok {
		tfMap["forward"] = flattenForwardAction(v)
	}

	return tfMap
}

func flattenRuleActionMemberFixedResponse(apiObject *types.RuleActionMemberFixedResponse) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.Value.StatusCode; v != nil {
		tfMap["status_code"] = aws.ToInt32(v)
	}

	return tfMap
}

func flattenForwardAction(apiObject *types.RuleActionMemberForward) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.Value.TargetGroups; v != nil {
		tfMap["forward"] = flattenWeightedTargetGroups(v)
	}

	return tfMap
}

func flattenWeightedTargetGroups(apiObjects []types.WeightedTargetGroup) []interface{} {
	if len(apiObjects) == 0 {
		return nil
	}

	var tfList []interface{}

	for _, apiObject := range apiObjects {
		tfList = append(tfList, flattenWeightedTargetGroup(&apiObject))
	}

	return tfList
}

func flattenWeightedTargetGroup(apiObject *types.WeightedTargetGroup) map[string]interface{} {

	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.TargetGroupIdentifier; v != nil {
		tfMap["target_group_identifier"] = aws.ToString(v)
	}

	if v := apiObject.Weight; v != nil {
		tfMap["weight"] = aws.ToInt32(v)
	}

	return tfMap
}

func flattenRuleMatch(apiObject types.RuleMatch) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := make(map[string]interface{})

	if v, ok := apiObject.(*types.RuleMatchMemberHttpMatch); ok {
		tfMap["http_match"] = flattenHttpMatch(&v.Value)
	}

	return tfMap
}

func flattenHttpMatch(apiObject *types.HttpMatch) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.Method; v != nil {
		tfMap["method"] = aws.ToString(v)
	}

	if v := apiObject.HeaderMatches; v != nil {
		tfMap["headers_matches"] = []interface{}{flattenHeaderMatches(v)}
	}

	if v := apiObject.PathMatch; v != nil {
		tfMap["path_match"] = []interface{}{flattenPathMatch(v)}
	}

	return tfMap
}

func flattenHeaderMatches(apiObjects []types.HeaderMatch) []interface{} {
	if len(apiObjects) == 0 {
		return nil
	}

	var tfList []interface{}

	for _, apiObject := range apiObjects {
		tfList = append(tfList, flattenHeaderMatch(&apiObject))
	}

	return tfList
}

func flattenHeaderMatch(apiObject *types.HeaderMatch) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.CaseSensitive; v != nil {
		tfMap["case_sensitive"] = aws.ToBool(v)
	}

	if v := apiObject.Name; v != nil {
		tfMap["name"] = aws.ToString(v)
	}

	if v := apiObject.Match; v != nil {
		if exact, ok := v.(*types.HeaderMatchTypeMemberExact); ok {
			tfMap["exact"] = flattenHeaderMatchTypeMemberExact(exact)
		}
		if prefix, ok := v.(*types.HeaderMatchTypeMemberPrefix); ok {
			tfMap["prefix"] = flattenHeaderMatchTypeMemberPrefix(prefix)
		}
		if contains, ok := v.(*types.HeaderMatchTypeMemberContains); ok {
			tfMap["contains"] = flattenHeaderMatchTypeMemberContains(contains)
		}
	}

	return tfMap
}

// func flattenHeaderMatchType(apiObject types.HeaderMatchType) map[string]interface{} {
// 	if apiObject == nil {
// 		return nil
// 	}

// 	tfMap := make(map[string]interface{})

// 	if v, ok := apiObject.(*types.HeaderMatchTypeMemberExact); ok {
// 		tfMap["exact"] = []interface{}{flattenHeaderMatchTypeMemberExact(v)}
// 	}

// 	if v, ok := apiObject.(*types.HeaderMatchTypeMemberPrefix); ok {
// 		tfMap["prefix"] = []interface{}{flattenHeaderMatchTypeMemberPrefix(v)}
// 	}

// 	if v, ok := apiObject.(*types.HeaderMatchTypeMemberContains); ok {
// 		tfMap["contains"] = []interface{}{flattenHeaderMatchTypeMemberContains(v)}
// 	}

// 	return tfMap
// }

func flattenHeaderMatchTypeMemberContains(apiObject *types.HeaderMatchTypeMemberContains) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{
		"contains": apiObject.Value,
	}

	return tfMap
}

func flattenHeaderMatchTypeMemberExact(apiObject *types.HeaderMatchTypeMemberExact) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{
		"exact": apiObject.Value,
	}

	return tfMap
}

func flattenHeaderMatchTypeMemberPrefix(apiObject *types.HeaderMatchTypeMemberPrefix) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{
		"prefix": apiObject.Value,
	}

	return tfMap
}

func flattenPathMatch(apiObject *types.PathMatch) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{}

	if v := apiObject.CaseSensitive; v != nil {
		tfMap["case_sensitive"] = aws.ToBool(v)
	}

	if v := apiObject.Match; v != nil {
		if exact, ok := v.(*types.PathMatchTypeMemberExact); ok {
			tfMap["exact"] = flattenPathMatchTypeMemberExact(exact)
		}
		if prefix, ok := v.(*types.PathMatchTypeMemberPrefix); ok {
			tfMap["prefix"] = flattenPathMatchTypeMemberPrefix(prefix)
		}
	}

	return tfMap
}

func flattenPathMatchTypeMemberExact(apiObject *types.PathMatchTypeMemberExact) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{
		"exact": apiObject.Value,
	}

	return tfMap
}

func flattenPathMatchTypeMemberPrefix(apiObject *types.PathMatchTypeMemberPrefix) map[string]interface{} {
	if apiObject == nil {
		return nil
	}

	tfMap := map[string]interface{}{
		"prefix": apiObject.Value,
	}

	return tfMap
}

func expandRuleAction(tfMap map[string]interface{}) types.RuleAction {
	var apiObject types.RuleAction

	if v, ok := tfMap["fixed_response_action"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		apiObject = expandFixedResponseAction(v[0].(map[string]interface{}))
	}
	if v, ok := tfMap["forward_action"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		apiObject = expandForwardAction(v[0].(map[string]interface{}))
	}

	return apiObject
}

func expandFixedResponseAction(tfMap map[string]interface{}) *types.RuleActionMemberFixedResponse {
	apiObject := &types.RuleActionMemberFixedResponse{}

	if v, ok := tfMap["status"].(int); ok && v != 0 {
		apiObject.Value.StatusCode = aws.Int32(int32(v))
	}

	return apiObject
}

func expandForwardAction(tfMap map[string]interface{}) *types.RuleActionMemberForward {
	apiObject := &types.RuleActionMemberForward{}

	if v, ok := tfMap["target_groups"].([]interface{}); ok && len(v) > 0 && v != nil {
		apiObject.Value.TargetGroups = expandWeightedTargetGroups(v)
	}

	return apiObject
}

func expandWeightedTargetGroups(tfList []interface{}) []types.WeightedTargetGroup {
	if len(tfList) == 0 {
		return nil
	}

	var apiObjects []types.WeightedTargetGroup

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]interface{})

		if !ok {
			continue
		}

		apiObject := expandWeightedTargetGroup(tfMap)

		apiObjects = append(apiObjects, apiObject)
	}

	return apiObjects
}

func expandWeightedTargetGroup(tfMap map[string]interface{}) types.WeightedTargetGroup {
	apiObject := types.WeightedTargetGroup{}

	if v, ok := tfMap["target_group_identifier"].(string); ok && v != "" {
		apiObject.TargetGroupIdentifier = aws.String(v)
	}

	if v, ok := tfMap["weight"].(int); ok && v != 0 {
		apiObject.Weight = aws.Int32(int32(v))
	}

	return apiObject
}

func expandRuleMatch(tfMap map[string]interface{}) types.RuleMatch {
	apiObject := &types.RuleMatchMemberHttpMatch{}

	if v, ok := tfMap["match"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		apiObject.Value = expandHttpMatch(v[0].(map[string]interface{}))
	}

	return apiObject
}

func expandHttpMatch(tfMap map[string]interface{}) types.HttpMatch {
	apiObject := types.HttpMatch{}

	if v, ok := tfMap["header_matches"].([]interface{}); ok && len(v) > 0 && v != nil {
		apiObject.HeaderMatches = expandHeaderMatches(v)
	}

	if v, ok := tfMap["method"].(string); ok {
		apiObject.Method = aws.String(v)
	}

	if v, ok := tfMap["matcher"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		apiObject.PathMatch = expandPathMatch(v[0].(map[string]interface{}))
	}

	return apiObject
}

func expandHeaderMatches(tfList []interface{}) []types.HeaderMatch {
	if len(tfList) == 0 {
		return nil
	}

	var apiObjects []types.HeaderMatch

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]interface{})

		if !ok {
			continue
		}

		apiObject := expandHeaderMatch(tfMap)

		apiObjects = append(apiObjects, apiObject)
	}

	return apiObjects
}

func expandHeaderMatch(tfMap map[string]interface{}) types.HeaderMatch {
	apiObject := types.HeaderMatch{}

	if v, ok := tfMap["case_sensitive"].(bool); ok {
		apiObject.CaseSensitive = aws.Bool(v)
	}

	if v, ok := tfMap["name"].(string); ok {
		apiObject.Name = aws.String(v)
	}

	if v, ok := tfMap["match"].([]interface{}); ok && len(v) > 0 {
		matchObj := v[0].(map[string]interface{})
		if matchV, ok := matchObj["exact"].(string); ok && matchV != "" {
			apiObject.Match = expandHeaderMatchTypeMemberExact(matchObj)
		}
		if matchV, ok := matchObj["prefix"].(string); ok && matchV != "" {
			apiObject.Match = expandHeaderMatchTypeMemberPrefix(matchObj)
		}
		if matchV, ok := matchObj["contains"].(string); ok && matchV != "" {
			apiObject.Match = expandHeaderMatchTypeMemberContains(matchObj)
		}
	}

	return apiObject
}

func expandHeaderMatchTypeMemberContains(tfMap map[string]interface{}) types.HeaderMatchType {
	apiObject := &types.HeaderMatchTypeMemberContains{}

	if v, ok := tfMap["contains"].(string); ok && v != "" {
		apiObject.Value = v
	}
	return apiObject
}

func expandHeaderMatchTypeMemberPrefix(tfMap map[string]interface{}) types.HeaderMatchType {
	apiObject := &types.HeaderMatchTypeMemberPrefix{}

	if v, ok := tfMap["prefix"].(string); ok && v != "" {
		apiObject.Value = v
	}
	return apiObject
}

func expandHeaderMatchTypeMemberExact(tfMap map[string]interface{}) types.HeaderMatchType {
	apiObject := &types.HeaderMatchTypeMemberExact{}

	if v, ok := tfMap["exact"].(string); ok && v != "" {
		apiObject.Value = v
	}
	return apiObject
}

func expandPathMatch(tfMap map[string]interface{}) *types.PathMatch {
	apiObject := &types.PathMatch{}

	if v, ok := tfMap["case_sensitive"].(bool); ok {
		apiObject.CaseSensitive = aws.Bool(v)
	}

	if v, ok := tfMap["match"].([]interface{}); ok && len(v) > 0 {
		matchObj := v[0].(map[string]interface{})

		if matchV, ok := matchObj["exact"].(string); ok && matchV != "" {
			apiObject.Match = expandPathMatchTypeMemberExact(matchObj)
		}

		if matchV, ok := matchObj["prefix"].(string); ok && matchV != "" {
			apiObject.Match = expandPathMatchTypeMemberPrefix(matchObj)
		}
	}

	return apiObject
}

func expandPathMatchTypeMemberExact(tfMap map[string]interface{}) types.PathMatchType {
	apiObject := &types.PathMatchTypeMemberExact{}

	if v, ok := tfMap["exact"].(string); ok && v != "" {
		apiObject.Value = v
	}
	return apiObject
}

func expandPathMatchTypeMemberPrefix(tfMap map[string]interface{}) types.PathMatchType {
	apiObject := &types.PathMatchTypeMemberPrefix{}

	if v, ok := tfMap["prefix"].(string); ok && v != "" {
		apiObject.Value = v
	}
	return apiObject
}
