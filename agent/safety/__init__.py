"""Safety policy package.

Public surface: assess_tool_call, SafetyDecision.
"""

from agent.safety._filesystem import is_dangerous_path as _is_dangerous_path
from agent.safety._filesystem import is_path_within_workspace as _is_path_within_workspace
from agent.safety._policy import SafetyDecision, assess_tool_call
from agent.safety._powershell import classify as _classify_powershell

__all__ = [
    "assess_tool_call",
    "SafetyDecision",
    "_is_path_within_workspace",
    "_is_dangerous_path",
    "_classify_powershell",
]
