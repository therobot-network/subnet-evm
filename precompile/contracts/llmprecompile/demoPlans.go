package llmprecompile


var demoPlans = map[string][]Step{
	"basic": {
	{
		Method:   "increase",
		Contract: Arg{
			Lookup: "USDC" ,
		},
		Args: []Arg{
			{
				Value: "10",
			},
		},
	},
	{
		Method:   "increase",
		Contract: Arg{
			Value: "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e",
		},
		Args: []Arg{
			{
				Value: "20",
			},
		},
	},
  },
  "withLookup": {
    {
      Method:   "increase",
      Contract: Arg{
				Value: "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
			},
	  Args: []Arg{
        {
          Value: "10",
        },
      },
    },
		{
			Method:   "getCounter",
			Contract: Arg{
				Value: "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
			},
			Args: []Arg{},
			Output: "CountA",
		},
		{
			Method:   "increase",
			Contract: Arg{
				Value: "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e",
			},
			Args: []Arg{
				{
					Lookup: "CountA",
				},
			},
		},
	},
  "withJumpIfNot": {
	},
	"erc20Plan": {
		{
			Method:   "balanceOf",
			Contract: Arg{
				Value: "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922",
			},
				Args: []Arg{
				{
					Value: "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				},
			},
		},
		{
			Method:   "divide",
			Contract: Arg{
				Value: "0xC6d7eF1e8BEd05586A46Bef5e1E392DF64070503",
			},
				Args: []Arg{
				{
					Lookup: "hello",
				},
				{
					Value: "2",
				},
			},
		},
		{
			Method:   "isLessThanOrEqual",
			Contract: Arg{
				Value: "0xC6d7eF1e8BEd05586A46Bef5e1E392DF64070503",
			},
			Args: []Arg{
				{
					Lookup: "hello",
				},
				{
					Value:  "10",
				},
			},
		},
		{
			Method:    "JumpIfNot",
		},
		{
			Method:   "JumpIfNot",
			Args: []Arg{
				{
					Value:  "true",
				},
				{
					Lookup: "hello",
				},
			},
		},
		{
			Method:   "transfer",
			Contract: Arg{
				Value: "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922",
			},
				Args: []Arg{
				{
					Value:  "0xad660da80c8D32E1a4Fb8DF6925A428060b58616",
				},
				{
					Lookup: "hello",
				},
			},
		},
	},
}
