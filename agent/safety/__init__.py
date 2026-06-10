"""Safety policy package.

Public surface: assess_tool_call, SafetyDecision.
"""

from agent.safety._policy import assess_tool_call, SafetyDecision

__all__ = ["assess_tool_call", "SafetyDecision"]
