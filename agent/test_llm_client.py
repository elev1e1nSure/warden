from agent.llm_client import OpenAIClient


def test_normalize_messages_preserves_reasoning_fields() -> None:
	client = OpenAIClient.__new__(OpenAIClient)
	messages = [
		{
			"role": "assistant",
			"content": "done",
			"tool_calls": [
				{
					"id": "call_1",
					"function": {"name": "read", "arguments": "{}"},
				}
			],
			"reasoning": "step by step",
			"reasoning_details": [{"type": "reasoning.text", "text": "step by step"}],
		}
	]

	result = OpenAIClient._normalize_messages(client, messages)

	assert result[0]["reasoning"] == "step by step"
	assert result[0]["reasoning_details"] == [{"type": "reasoning.text", "text": "step by step"}]
	assert result[0]["tool_calls"][0]["function"]["name"] == "read"
