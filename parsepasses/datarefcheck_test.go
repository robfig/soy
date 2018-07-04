package parsepasses

import (
	"testing"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/template"
)

type checkerTest struct {
	body    []string
	success bool
}

type simpleCheckerTest struct {
	body    string
	success bool
}

// Test: all data references are provided by @param declarations or {let},
// {for}, {foreach} nodes
func TestAllDataRefsProvided(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** no data refs */
{template .noDataRefs}
Hello world
{/template}`, true},

		{`
/** let only */
{template .letOnly}
{let $ref: 0/}Hello {$ref}
{/template}`, true},

		{`
/** @param paramName */
{template .paramOnly}
Hello {$paramName}
{/template}`, true},

		{`
/**
 * @param param1
 * @param? param2
 */
{template .everything}
{let $let1: 'hello'/}
{if true}{let $let2}let body{/let}
Hello {$param1} {$param2} {$let1} {$let2}
{else}
Goodbye {$param1} {$param2} {$let1}
{/if}
{/template}`, true},

		{`
/** for loop */
{template .for}
{for $x in range(5)}
  Hello {$x}
{/for}
{/template}`, true},

		{`
/** @param vars */
{template .foreach}
{foreach $x in $vars}
  Hello world
{/for}
{/template}`, true},

		{`
/** missing param */
{template .missingParam}
Hello {$param}
{/template}`, false},

		{`
/** out of scope */
{template .letOutOfScope}
{if true}
{let $param: true/}
Hello {$param}
{/if}
Hello {$param}
{/template}`, false},

		{`
/** @param foo (within expression) */
{template .for}
{foreach $x in $foo[$bar]}
  Hello {$x}
{/for}
{/template}`, false},
	})

}

// Test: any data declared as a @param is used by the template (or passed via {call})
func TestAllParamsAreUsed(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{

		// Check successful
		{`
/** @param used */
{template .ParamUsedInExpr}
  Hello {not true ? 'a' : 'b' + $used}.
{/template}`, true},

		{`
/** @param param */
{template .UsedInCallData}
  Hello {call .Other data="$param"/}.
{/template}
/** No params */
{template .Other}
{/template}`, true},

		{`
/** @param param */
{template .PassedByCall_AllData}
  Hello {call .Other data="all"/}.
{/template}
/** @param param */
{template .Other}
  Hello {$param}
{/template}`, true},

		{`
/**
 * @param foo
 * @param bar
 */
{template .for}
{foreach $x in $foo[$bar]}
  Hello {$x}
{/for}
{/template}`, true},

		// Check fails
		{`
/** @param param not used */
{template .ParamNotUsed}
  Hello {call .Other/}
{/template}
/** @param? param not used */
{template .Other}
  {$param}
{/template}`, false},

		{`
/**
 * @param used
 * @param notused
 */
{template .CallPassesAllDataButNotDeclaredByCallee}
  Hello {call .Other data="all"/}.
{/template}
/**
 * @param used
 * @param? other
 */
{template .Other}
 {$used}
{/template}`, false},

		{`
/** @param var */
{template .ParamShadowedByLet}
  {let $var: 0/}
  Hello {$var}
{/template}`, false},
	})
}

// Test: all {call} params are declared as @params in the called template soydoc.
func TestAllCallParamsAreDeclaredByCallee(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** */
{template .ParamsPresent}
  {call .Other}
    {param param1: 0/}
    {param param2}hello{/param}
  {/call}
{/template}
/**
 * @param param1
 * @param? param2
 */
{template .Other}
  {$param1} {$param2}
{/template}
`, true},

		{`
/** */
{template .ParamsNotPresent}
  {call .Other}
    {param param1}hello{/param}
  {/call}
{/template}
/** */
{template .Other}
{/template}
`, false},
	})
}

// Test: a {call}'ed template is passed all required @params, or a data="$var"
func TestCalledTemplatesReceiveAllRequiredParams(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** */
{template .NotPassingRequiredParam}
  {call .Other/}
{/template}
/** @param required */
{template .Other}
  {$required}
{/template}
`, false},

		{`
/** @param required */
{template .PassingRequiredParam_AllData}
  {call .Other data="all"/}
{/template}
/** @param required */
{template .Other}
  {$required}
{/template}
`, true},
		{`
/** */
{template .NotPassingRequiredParam_AllData}
  {call .Other data="all"/}
{/template}
/** @param required */
{template .Other}
  {$required}
{/template}
`, false},

		{`
/** @param something */
{template .PassingRequiredParam_OneData}
  {call .Other data="$something"/}
{/template}
/** @param required */
{template .Other}
  {$required}
{/template}
`, true},

		{`
/** @param something */
{template .PassingRequiredParam_AsParam}
  {call .Other}
    {param required: $something/}
  {/call}
{/template}
/** @param required */
{template .Other}
  {$required}
{/template}
`, true},
	})
}

// Test: {call}'d templates actually exist in the registry.
func TestCalledTemplatesRequiredToExist(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** @param var */
{template .CalledTemplateDoesNotExist}
{call .NotExist data="$var"/}
{/template}
`, false},
	})
}

// Test: any variable created by {let}, {for}, {foreach} is used somewhere
func TestLetVariablesAreUsed(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** */
{template .UnusedLetVariable}
{let $var}hello{/let}
Hello world.
{/template}
`, false},

		{`
/** @param var */
{template .UnusedShadowingLetVariable}
{if true}
 {let $var}hello{/let}
{/if}
Hello {$var}
{/template}
`, false},
	})
}

// Test that {call} checks work on calls across namespaces too.
func TestCrossNamespace(t *testing.T) {
	runCheckerTests(t, []checkerTest{
		{[]string{`
{namespace ns.a}
/** */
{template .ParamsPresent}
  {call ns.b.Other}
    {param param1: 0/}
    {param param2}hello{/param}
  {/call}
{/template}
`, `
{namespace ns.b}
/**
 * @param param1
 * @param? param2
 */
{template .Other}
  {$param1} {$param2}
{/template}
`}, true},

		{[]string{`
{namespace ns.a}
/** */
{template .ParamsNotPresent}
  {call ns.b.Other}
    {param param1}hello{/param}
  {/call}
{/template}
`, `
{namespace ns.b}
/** */
{template .Other}
{/template}
`}, false},
	})
}

// Test: {let} variables are not named $ij
func TestLetVariablesNotNamedIJ(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** */
{template .noCollideIJ}
{let $ij}hello{/let}
Hello {$ij}
{/template}
`, false},
	})
}

// Test: $ij named variables are allowed without declaration
func TestIJVarsAllowed(t *testing.T) {
	runSimpleCheckerTests(t, []simpleCheckerTest{
		{`
/** */
{template .IJAllowed}
{$ij.foo}
{/template}
`, true},
	})
}

func runSimpleCheckerTests(t *testing.T, tests []simpleCheckerTest) {
	var result []checkerTest
	for _, simpleTest := range tests {
		result = append(result, checkerTest{
			[]string{"{namespace test}\n" + simpleTest.body},
			simpleTest.success,
		})
	}
	runCheckerTests(t, result)
}

func runCheckerTests(t *testing.T, tests []checkerTest) {
	for _, test := range tests {
		var (
			reg  template.Registry
			tree *ast.SoyFileNode
			err  error
		)
		for _, body := range test.body {
			tree, err = parse.SoyFile("", body)
			if err != nil {
				t.Error(err)
				continue
			}

			if err := reg.Add(tree); err != nil {
				t.Error(err)
				continue
			}
		}

		err = CheckDataRefs(reg)
		if test.success && err != nil {
			t.Error(err)
		} else if !test.success && err == nil {
			t.Errorf("%s: expected to fail validation, but no error was raised.",
				reg.Templates[0].Node.Name)
		}
	}
}
