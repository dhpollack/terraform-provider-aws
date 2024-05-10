// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wafregional

import (
	"fmt"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafregional"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wafregional/types"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/awsv2"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/sdk"
)

func RegisterSweepers() {
	resource.AddTestSweepers("aws_wafregional_rate_based_rule", &resource.Sweeper{
		Name: "aws_wafregional_rate_based_rule",
		F:    sweepRateBasedRules,
		Dependencies: []string{
			"aws_wafregional_web_acl",
		},
	})

	resource.AddTestSweepers("aws_wafregional_regex_match_set", &resource.Sweeper{
		Name: "aws_wafregional_regex_match_set",
		F:    sweepRegexMatchSets,
	})

	resource.AddTestSweepers("aws_wafregional_regex_pattern_set", &resource.Sweeper{
		Name: "aws_wafregional_regex_pattern_set",
		F:    sweepRegexPatternSets,
	})

	resource.AddTestSweepers("aws_wafregional_rule_group", &resource.Sweeper{
		Name: "aws_wafregional_rule_group",
		F:    sweepRuleGroups,
	})

	resource.AddTestSweepers("aws_wafregional_rule", &resource.Sweeper{
		Name: "aws_wafregional_rule",
		F:    sweepRules,
		Dependencies: []string{
			"aws_wafregional_web_acl",
		},
	})

	resource.AddTestSweepers("aws_wafregional_web_acl", &resource.Sweeper{
		Name: "aws_wafregional_web_acl",
		F:    sweepWebACLs,
	})
}

func sweepRateBasedRules(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.WAFRegionalClient(ctx)

	input := &wafregional.ListRateBasedRulesInput{}

	for {
		output, err := conn.ListRateBasedRules(ctx, input)

		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping WAF Regional Rate-Based Rule sweep for %s: %s", region, err)
			return nil
		}

		if err != nil {
			return fmt.Errorf("error listing WAF Regional Rate-Based Rules: %s", err)
		}

		for _, rule := range output.Rules {
			deleteInput := &wafregional.DeleteRateBasedRuleInput{
				RuleId: rule.RuleId,
			}
			id := aws.ToString(rule.RuleId)
			wr := newRetryer(conn, region)

			_, err := wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
				deleteInput.ChangeToken = token
				log.Printf("[INFO] Deleting WAF Regional Rate-Based Rule: %s", id)
				return conn.DeleteRateBasedRule(ctx, deleteInput)
			})

			if errs.IsA[*awstypes.WAFNonEmptyEntityException](err) {
				getRateBasedRuleInput := &wafregional.GetRateBasedRuleInput{
					RuleId: rule.RuleId,
				}

				getRateBasedRuleOutput, getRateBasedRuleErr := conn.GetRateBasedRule(ctx, getRateBasedRuleInput)

				if getRateBasedRuleErr != nil {
					return fmt.Errorf("error getting WAF Regional Rate-Based Rule (%s): %s", id, getRateBasedRuleErr)
				}

				var updates []awstypes.RuleUpdate
				updateRateBasedRuleInput := &wafregional.UpdateRateBasedRuleInput{
					RateLimit: getRateBasedRuleOutput.Rule.RateLimit,
					RuleId:    rule.RuleId,
					Updates:   updates,
				}

				for _, predicate := range getRateBasedRuleOutput.Rule.MatchPredicates {
					update := awstypes.RuleUpdate{
						Action:    awstypes.ChangeActionDelete,
						Predicate: &predicate,
					}

					updateRateBasedRuleInput.Updates = append(updateRateBasedRuleInput.Updates, update)
				}

				_, updateWebACLErr := wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
					updateRateBasedRuleInput.ChangeToken = token
					log.Printf("[INFO] Removing Predicates from WAF Regional Rate-Based Rule: %s", id)
					return conn.UpdateRateBasedRule(ctx, updateRateBasedRuleInput)
				})

				if updateWebACLErr != nil {
					return fmt.Errorf("error removing predicates from WAF Regional Rate-Based Rule (%s): %s", id, updateWebACLErr)
				}

				_, err = wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
					deleteInput.ChangeToken = token
					log.Printf("[INFO] Deleting WAF Regional Rate-Based Rule: %s", id)
					return conn.DeleteRateBasedRule(ctx, deleteInput)
				})
			}

			if err != nil {
				return fmt.Errorf("error deleting WAF Regional Rate-Based Rule (%s): %s", id, err)
			}
		}

		if aws.ToString(output.NextMarker) == "" {
			break
		}

		input.NextMarker = output.NextMarker
	}

	return nil
}

func sweepRegexMatchSets(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.WAFRegionalClient(ctx)
	input := &wafregional.ListRegexMatchSetsInput{}
	sweepResources := make([]sweep.Sweepable, 0)

	err = listRegexMatchSetsPages(ctx, conn, input, func(page *wafregional.ListRegexMatchSetsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, v := range page.RegexMatchSets {
			id := aws.ToString(v.RegexMatchSetId)

			v, err := findRegexMatchSetByID(ctx, conn, id)

			if err != nil {
				continue
			}

			r := resourceRegexMatchSet()
			d := r.Data(nil)
			d.SetId(id)
			d.Set("regex_match_tuple", flattenRegexMatchTuples(v.RegexMatchTuples))

			sweepResources = append(sweepResources, sweep.NewSweepResource(r, d, client))
		}

		return !lastPage
	})

	if awsv2.SkipSweepError(err) {
		log.Printf("[WARN] Skipping WAF Regional RegexMatchSet sweep for %s: %s", region, err)
		return nil
	}

	if err != nil {
		return fmt.Errorf("error listing WAF Regional RegexMatchSets (%s): %w", region, err)
	}

	err = sweep.SweepOrchestrator(ctx, sweepResources)

	if err != nil {
		return fmt.Errorf("error sweeping WAF Regional RegexMatchSets (%s): %w", region, err)
	}

	return nil
}

func sweepRegexPatternSets(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.WAFRegionalClient(ctx)
	input := &wafregional.ListRegexPatternSetsInput{}
	sweepResources := make([]sweep.Sweepable, 0)

	err = listRegexPatternSetsPages(ctx, conn, input, func(page *wafregional.ListRegexPatternSetsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, v := range page.RegexPatternSets {
			id := aws.ToString(v.RegexPatternSetId)

			v, err := findRegexPatternSetByID(ctx, conn, id)

			if err != nil {
				continue
			}

			r := resourceRegexPatternSet()
			d := r.Data(nil)
			d.SetId(id)
			d.Set("regex_pattern_strings", aws.StringSlice(v.RegexPatternStrings))

			sweepResources = append(sweepResources, sweep.NewSweepResource(r, d, client))
		}

		return !lastPage
	})

	if awsv2.SkipSweepError(err) {
		log.Printf("[WARN] Skipping WAF Regional RegexPatternSet sweep for %s: %s", region, err)
		return nil
	}

	if err != nil {
		return fmt.Errorf("error listing WAF Regional RegexPatternSets (%s): %w", region, err)
	}

	err = sweep.SweepOrchestrator(ctx, sweepResources)

	if err != nil {
		return fmt.Errorf("error sweeping WAF Regional RegexPatternSets (%s): %w", region, err)
	}

	return nil
}

func sweepRuleGroups(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.WAFRegionalClient(ctx)

	sweepResources := make([]sweep.Sweepable, 0)
	var errs *multierror.Error
	var g multierror.Group
	var mutex = &sync.Mutex{}

	input := &wafregional.ListRuleGroupsInput{}

	err = listRuleGroupsPages(ctx, conn, input, func(page *wafregional.ListRuleGroupsOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, ruleGroup := range page.RuleGroups {
			r := resourceRuleGroup()
			d := r.Data(nil)

			id := aws.ToString(ruleGroup.RuleGroupId)
			d.SetId(id)

			// read concurrently and gather errors
			g.Go(func() error {
				// Need to Read first to fill in activated_rule attribute
				err := sdk.ReadResource(ctx, r, d, client)

				if err != nil {
					sweeperErr := fmt.Errorf("error reading WAF Regional Rule Group (%s): %w", id, err)
					log.Printf("[ERROR] %s", sweeperErr)
					return sweeperErr
				}

				// In case it was already deleted
				if d.Id() == "" {
					return nil
				}

				mutex.Lock()
				defer mutex.Unlock()
				sweepResources = append(sweepResources, sweep.NewSweepResource(r, d, client))

				return nil
			})
		}

		return !lastPage
	})

	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error listing WAF Regional Rule Group for %s: %w", region, err))
	}

	if err = g.Wait().ErrorOrNil(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error concurrently reading WAF Regional Rule Groups: %w", err))
	}

	if err = sweep.SweepOrchestrator(ctx, sweepResources); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("error sweeping WAF Regional Rule Group for %s: %w", region, err))
	}

	if awsv2.SkipSweepError(errs.ErrorOrNil()) {
		log.Printf("[WARN] Skipping WAF Regional Rule Group sweep for %s: %s", region, errs)
		return nil
	}

	return errs.ErrorOrNil()
}

func sweepRules(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.WAFRegionalClient(ctx)

	input := &wafregional.ListRulesInput{}

	for {
		output, err := conn.ListRules(ctx, input)

		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping WAF Regional Rule sweep for %s: %s", region, err)
			return nil
		}

		if err != nil {
			return fmt.Errorf("error listing WAF Regional Rules: %s", err)
		}

		for _, rule := range output.Rules {
			deleteInput := &wafregional.DeleteRuleInput{
				RuleId: rule.RuleId,
			}
			id := aws.ToString(rule.RuleId)
			wr := newRetryer(conn, region)

			_, err := wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
				deleteInput.ChangeToken = token
				log.Printf("[INFO] Deleting WAF Regional Rule: %s", id)
				return conn.DeleteRule(ctx, deleteInput)
			})

			if errs.IsA[*awstypes.WAFNonEmptyEntityException](err) {
				getRuleInput := &wafregional.GetRuleInput{
					RuleId: rule.RuleId,
				}

				getRuleOutput, getRuleErr := conn.GetRule(ctx, getRuleInput)

				if getRuleErr != nil {
					return fmt.Errorf("error getting WAF Regional Rule (%s): %s", id, getRuleErr)
				}

				var updates []awstypes.RuleUpdate
				updateRuleInput := &wafregional.UpdateRuleInput{
					RuleId:  rule.RuleId,
					Updates: updates,
				}

				for _, predicate := range getRuleOutput.Rule.Predicates {
					update := &awstypes.RuleUpdate{
						Action:    awstypes.ChangeActionDelete,
						Predicate: &predicate,
					}

					updateRuleInput.Updates = append(updateRuleInput.Updates, *update)
				}

				_, updateWebACLErr := wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
					updateRuleInput.ChangeToken = token
					log.Printf("[INFO] Removing Predicates from WAF Regional Rule: %s", id)
					return conn.UpdateRule(ctx, updateRuleInput)
				})

				if updateWebACLErr != nil {
					return fmt.Errorf("error removing predicates from WAF Regional Rule (%s): %s", id, updateWebACLErr)
				}

				_, err = wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
					deleteInput.ChangeToken = token
					log.Printf("[INFO] Deleting WAF Regional Rule: %s", id)
					return conn.DeleteRule(ctx, deleteInput)
				})
			}

			if err != nil {
				return fmt.Errorf("error deleting WAF Regional Rule (%s): %s", id, err)
			}
		}

		if aws.ToString(output.NextMarker) == "" {
			break
		}

		input.NextMarker = output.NextMarker
	}

	return nil
}

func sweepWebACLs(region string) error {
	ctx := sweep.Context(region)
	client, err := sweep.SharedRegionalSweepClient(ctx, region)
	if err != nil {
		return fmt.Errorf("error getting client: %s", err)
	}
	conn := client.WAFRegionalClient(ctx)

	input := &wafregional.ListWebACLsInput{}

	for {
		output, err := conn.ListWebACLs(ctx, input)

		if awsv2.SkipSweepError(err) {
			log.Printf("[WARN] Skipping WAF Regional Web ACL sweep for %s: %s", region, err)
			return nil
		}

		if err != nil {
			return fmt.Errorf("error listing WAF Regional Web ACLs: %s", err)
		}

		for _, webACL := range output.WebACLs {
			deleteInput := &wafregional.DeleteWebACLInput{
				WebACLId: webACL.WebACLId,
			}
			id := aws.ToString(webACL.WebACLId)
			wr := newRetryer(conn, region)

			_, err := wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
				deleteInput.ChangeToken = token
				log.Printf("[INFO] Deleting WAF Regional Web ACL: %s", id)
				return conn.DeleteWebACL(ctx, deleteInput)
			})

			if errs.IsA[*awstypes.WAFNonEmptyEntityException](err) {
				getWebACLInput := &wafregional.GetWebACLInput{
					WebACLId: webACL.WebACLId,
				}

				getWebACLOutput, getWebACLErr := conn.GetWebACL(ctx, getWebACLInput)

				if getWebACLErr != nil {
					return fmt.Errorf("error getting WAF Regional Web ACL (%s): %s", id, getWebACLErr)
				}

				var updates []awstypes.WebACLUpdate
				updateWebACLInput := &wafregional.UpdateWebACLInput{
					DefaultAction: getWebACLOutput.WebACL.DefaultAction,
					Updates:       updates,
					WebACLId:      webACL.WebACLId,
				}

				for _, rule := range getWebACLOutput.WebACL.Rules {
					update := &awstypes.WebACLUpdate{
						Action:        awstypes.ChangeActionDelete,
						ActivatedRule: &rule,
					}

					updateWebACLInput.Updates = append(updateWebACLInput.Updates, *update)
				}

				_, updateWebACLErr := wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
					updateWebACLInput.ChangeToken = token
					log.Printf("[INFO] Removing Rules from WAF Regional Web ACL: %s", id)
					return conn.UpdateWebACL(ctx, updateWebACLInput)
				})

				if updateWebACLErr != nil {
					return fmt.Errorf("error removing rules from WAF Regional Web ACL (%s): %s", id, updateWebACLErr)
				}

				_, err = wr.RetryWithToken(ctx, func(token *string) (interface{}, error) {
					deleteInput.ChangeToken = token
					log.Printf("[INFO] Deleting WAF Regional Web ACL: %s", id)
					return conn.DeleteWebACL(ctx, deleteInput)
				})
			}

			if err != nil {
				return fmt.Errorf("error deleting WAF Regional Web ACL (%s): %s", id, err)
			}
		}

		if aws.ToString(output.NextMarker) == "" {
			break
		}

		input.NextMarker = output.NextMarker
	}

	return nil
}
