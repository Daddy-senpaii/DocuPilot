import React from 'react';
import { DocumentItem, QAItem, QUICK_PROMPTS } from '../types';

interface ChatSectionProps {
  activeDoc: DocumentItem | null;
  text: string;
  question: string;
  onChangeQuestion: (question: string) => void;
  onAsk: (e: React.FormEvent) => void;
  loading: boolean;
  isStreaming: boolean;
  uploading: boolean;
  answer: string | null;
  sources: string[];
  history: QAItem[];
  copiedAnswer: boolean;
  onCopyAnswer: () => void;
}

export const ChatSection: React.FC<ChatSectionProps> = ({
  activeDoc,
  text,
  question,
  onChangeQuestion,
  onAsk,
  loading,
  isStreaming,
  uploading,
  answer,
  sources,
  history,
  copiedAnswer,
  onCopyAnswer,
}) => {
  return (
    <>
      <hr className="divider" />

      <form onSubmit={onAsk}>
        {(activeDoc || text.trim()) && (
          <div className="quick-prompts-bar">
            <span className="quick-prompts-label">✨ Quick Prompts:</span>
            <div className="quick-prompts-list">
              {QUICK_PROMPTS.map((prompt, idx) => (
                <button
                  key={idx}
                  type="button"
                  className="quick-prompt-btn"
                  onClick={() => onChangeQuestion(prompt)}
                  disabled={loading || uploading}
                >
                  {prompt}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="form-group">
          <label htmlFor="question">Ask a question about this document:</label>
          <input
            type="text"
            id="question"
            value={question}
            onChange={(e) => onChangeQuestion(e.target.value)}
            placeholder="e.g., What are the main recommendations?"
            disabled={loading || uploading}
            required
          />
        </div>

        <button
          type="submit"
          className="btn-primary"
          disabled={loading || uploading || (!text.trim() && !activeDoc) || !question.trim()}
        >
          {loading ? 'Searching & Generating Answer...' : 'Ask DocuPilot'}
        </button>
      </form>

      {(answer || isStreaming) && (
        <section className="answer-card" aria-live="polite">
          <div className="answer-header">
            <h2>
              💡 AI Grounded Answer{' '}
              {isStreaming && <span className="streaming-badge">Streaming live...</span>}
            </h2>
            {answer && !isStreaming && (
              <button
                type="button"
                className="btn-copy-answer"
                onClick={onCopyAnswer}
                title="Copy answer to clipboard"
              >
                {copiedAnswer ? '✓ Copied!' : '📋 Copy Answer'}
              </button>
            )}
          </div>
          <div className="answer-content">
            {answer || (isStreaming ? 'Searching passages and thinking...' : '')}
            {isStreaming && <span className="streaming-cursor">▊</span>}
          </div>

          {sources.length > 0 && (
            <details className="sources-section" open={isStreaming}>
              <summary>Retrieved Sources ({sources.length} passages)</summary>
              <div className="sources-list">
                {sources.map((s, idx) => (
                  <pre key={idx} className="source-item">{s}</pre>
                ))}
              </div>
            </details>
          )}
        </section>
      )}

      {history.length > 0 && (
        <section className="history-card">
          <h2>📜 Previous Questions & Answers</h2>
          <div className="history-list">
            {history.map((item) => (
              <div key={item.id} className="history-item">
                <p className="history-q"><strong>Q:</strong> {item.question}</p>
                <p className="history-a"><strong>A:</strong> {item.answer}</p>
              </div>
            ))}
          </div>
        </section>
      )}
    </>
  );
};
