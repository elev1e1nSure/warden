"""Characterization tests for ConfirmationManager and QuestionManager."""

import asyncio

import pytest

from agent.confirmations import ConfirmationManager, QuestionManager, _TIMEOUT_SECONDS


@pytest.fixture(autouse=True)
def short_timeout(monkeypatch):
    monkeypatch.setattr("agent.confirmations._TIMEOUT_SECONDS", 0.05)


# ---------------------------------------------------------------------------
# ConfirmationManager
# ---------------------------------------------------------------------------

class TestConfirmationManager:
    def test_register_returns_uuid_and_event(self) -> None:
        mgr = ConfirmationManager()
        call_id, event = mgr.register()
        assert isinstance(call_id, str)
        assert len(call_id) == 36  # UUID4
        assert isinstance(event, asyncio.Event)
        assert not event.is_set()

    def test_resolve_sets_event(self) -> None:
        mgr = ConfirmationManager()
        call_id, event = mgr.register()
        assert mgr.resolve(call_id, True)
        assert event.is_set()
        entry = mgr.pop(call_id)
        assert entry is not None
        assert entry["resolved"] is True
        assert entry["ok"] is True

    def test_resolve_duplicate_is_noop(self) -> None:
        mgr = ConfirmationManager()
        call_id, _ = mgr.register()
        assert mgr.resolve(call_id, True)
        assert not mgr.resolve(call_id, False)  # already resolved

    def test_wait_returns_true_when_confirmed(self) -> None:
        mgr = ConfirmationManager()
        call_id, _ = mgr.register()

        async def confirm_later():
            await asyncio.sleep(0.01)
            mgr.resolve(call_id, True)

        async def test():
            asyncio.create_task(confirm_later())
            result = await mgr.wait(call_id)
            assert result is True

        asyncio.run(test())

    def test_wait_returns_false_when_cancelled(self) -> None:
        mgr = ConfirmationManager()
        call_id, _ = mgr.register()

        async def test():
            result = await mgr.wait(call_id)
            assert result is False  # cancelled/timeout

        asyncio.run(test())

    def test_get_returns_none_after_expiry(self) -> None:
        mgr = ConfirmationManager()
        call_id, _ = mgr.register()
        import time
        time.sleep(0.1)
        assert mgr.get(call_id) is None

    def test_cancel_all_sets_all_events(self) -> None:
        mgr = ConfirmationManager()
        ids = [mgr.register()[0] for _ in range(3)]
        mgr.cancel_all()
        for cid in ids:
            entry = mgr.get(cid)
            assert entry is None


# ---------------------------------------------------------------------------
# QuestionManager
# ---------------------------------------------------------------------------

class TestQuestionManager:
    def test_register_stores_questions(self) -> None:
        mgr = QuestionManager()
        questions = [{"question": "q1", "options": []}]
        call_id, event = mgr.register(questions)
        assert isinstance(call_id, str)
        assert not event.is_set()
        entry = mgr.pop(call_id)
        assert entry is not None
        assert entry["questions"] == questions

    def test_resolve_sets_answers(self) -> None:
        mgr = QuestionManager()
        questions = [{"question": "q1", "options": ["a", "b"]}]
        call_id, _ = mgr.register(questions)
        answers = [["a"]]
        assert mgr.resolve(call_id, answers)
        entry = mgr.pop(call_id)
        assert entry is not None
        assert entry["resolved"] is True
        assert entry["answers"] == [["a"]]

    def test_wait_returns_answers(self) -> None:
        mgr = QuestionManager()
        questions = [{"question": "q1", "options": []}]
        call_id, _ = mgr.register(questions)

        async def answer_later():
            await asyncio.sleep(0.01)
            mgr.resolve(call_id, [["yes"]])

        async def test():
            asyncio.create_task(answer_later())
            result = await mgr.wait(call_id)
            assert result == [["yes"]]

        asyncio.run(test())

    def test_wait_returns_none_on_timeout(self) -> None:
        mgr = QuestionManager()
        questions = [{"question": "q1", "options": []}]
        call_id, _ = mgr.register(questions)

        async def test():
            result = await mgr.wait(call_id)
            assert result is None

        asyncio.run(test())

    def test_pop_preserves_answers_on_expired(self) -> None:
        mgr = QuestionManager()
        questions = [{"question": "q1", "options": []}]
        call_id, _ = mgr.register(questions)
        # pre-resolve with partial answers
        mgr.resolve(call_id, [["partial"]])

        import time
        time.sleep(0.1)
        entry = mgr.pop(call_id)
        # should preserve answers even if expired
        assert entry is not None
        assert entry["answers"] == [["partial"]]

    def test_pending_count_excludes_expired(self) -> None:
        mgr = QuestionManager()
        mgr.register([{"question": "q1"}])
        import time
        time.sleep(0.1)
        assert mgr.pending_count() == 0
