package llmprecompile


var demoPlans = map[string][]Step{
	"basic": {
	{
		Method:   "increase",
		Contract: "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
		ABI: `[
			{
				"inputs": [
					{
						"internalType": "uint256",
						"name": "number",
						"type": "uint256"
					}
				],
				"name": "increase",
				"outputs": [
					{
						"internalType": "uint256",
						"name": "",
						"type": "uint256"
					}
				],
				"stateMutability": "nonpayable",
				"type": "function"
			}
		]`,
		Args: []Arg{
			{
				Value: "10",
			},
		},
	},
	{
		Method:   "increase",
		Contract: "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e",
		ABI: `[
			{
				"inputs": [
					{
						"internalType": "uint256",
						"name": "number",
						"type": "uint256"
					}
				],
				"name": "increase",
				"outputs": [
					{
						"internalType": "uint256",
						"name": "",
						"type": "uint256"
					}
				],
				"stateMutability": "nonpayable",
				"type": "function"
			}
		]`,
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
      Contract: "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
      ABI: primitiveABI["counter"],
	  Args: []Arg{
        {
          Value: "10",
        },
      },
    },
		{
			Method:   "getCounter",
			Contract: "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
			ABI: primitiveABI["counter"],
			Args: []Arg{},
		},
		{
			Method:   "increase",
			Contract: "0x5aa01B3b5877255cE50cc55e8986a7a5fe29C70e",
			ABI: primitiveABI["counter"],
			Args: []Arg{
				{
					Lookup: true,
					PcLookupKey: 1,
					ReturnArgKey: 0,
				},
			},
		},
	},
  "withJumpIf": {
	},
	"erc20Plan": {
		{
			Method:   "balanceOf",
			Contract: "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922",
			ABI: `[
				{
					"inputs": [
						{
							"internalType": "address",
							"name": "account",
							"type": "address"
						}
					],
					"name": "balanceOf",
					"outputs": [
						{
							"internalType": "uint256",
							"name": "",
							"type": "uint256"
						}
					],
					"stateMutability": "view",
					"type": "function"
				}
			]`,
			Args: []Arg{
				{
					Value: "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				},
			},
		},
		{
			Method:   "divide",
			Contract: "0xC6d7eF1e8BEd05586A46Bef5e1E392DF64070503",
			ABI: `[
				{
					"inputs": [
						{
							"internalType": "uint256",
							"name": "a",
							"type": "uint256"
						},
						{
							"internalType": "uint256",
							"name": "b",
							"type": "uint256"
						}
					],
					"name": "divide",
					"outputs": [
						{
							"internalType": "uint256",
							"name": "",
							"type": "uint256"
						}
					],
					"stateMutability": "pure",
					"type": "function"
				}
			]`,
			Args: []Arg{
				{
					Lookup: true,
				},
				{
					Value: "2",
				},
			},
		},
		{
			Method:   "isLessThanOrEqual",
			Contract: "0xC6d7eF1e8BEd05586A46Bef5e1E392DF64070503",
			ABI: `[
				{
					"inputs": [
						{
							"internalType": "uint256",
							"name": "a",
							"type": "uint256"
						},
						{
							"internalType": "uint256",
							"name": "b",
							"type": "uint256"
						}
					],
					"name": "isLessThanOrEqual",
					"outputs": [
						{
							"internalType": "bool",
							"name": "",
							"type": "bool"
						}
					],
					"stateMutability": "pure",
					"type": "function"
				}
			]`,
			Args: []Arg{
				{
					Lookup: true,
				},
				{
					Value:  "10",
				},
			},
		},
		{
			PcStep:    true,
			Method:    "jumpIf",
			Condition: 2,
			SkipTo:    5,
		},
		{
			Method:   "transfer",
			Contract: "0x52C84043CD9c865236f11d9Fc9F56aa003c1f922",
			ABI: `[
				{
					"inputs": [
						{
							"internalType": "address",
							"name": "to",
							"type": "address"
						},
						{
							"internalType": "uint256",
							"name": "amount",
							"type": "uint256"
						}
					],
					"name": "transfer",
					"outputs": [
						{
							"internalType": "bool",
							"name": "",
							"type": "bool"
						}
					],
					"stateMutability": "nonpayable",
					"type": "function"
				}
			]`,
			Args: []Arg{
				{
					Value:  "0xad660da80c8D32E1a4Fb8DF6925A428060b58616",
				},
				{
					Lookup: true,
				},
			},
		},
	},
}
