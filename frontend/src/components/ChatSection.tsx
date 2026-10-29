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
  const hasContent = Boolean(activeDoc || text.trim());

  return (
    <>
      <hr className="divider" />

      <form onSubmit={onAsk} className="chat-form">
        {hasContent && (
          <div className="quick-prompts-bar">
            <span className="quick-prompts-label">
              <span className="sparkle-icon">✨</span>
              <span>Quick Prompts:</span>
            </span>
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

        <div className="form-group question-form-group">
          <label htmlFor="question">
            <span>Ask a question about this document:</span>
            {activeDoc && (
              <span className="active-target-badge">
                Target: {activeDoc.filename}
              </span>
            )}
          </label>
          <div className="question-input-wrapper">
            <input
              type="text"
              id="question"
              value={question}
              onChange={(e) => onChangeQuestion(e.target.value)}
              placeholder="e.g., What are the key takeaways and recommendations?"
              disabled={loading || uploading}
              required
            />
          </div>
        </div>

        <button
          type="submit"
          className="btn-primary btn-ask-pilot"
          disabled={loading || uploading || !hasContent || !question.trim()}
        >
          {loading ? (
            <span className="btn-loading-content">
              <span className="spinner" />
              <span>Searching Passages & Generating Answer...</span>
            </span>
          ) : (
            <span className="ask-btn-content">
              <span className="ask-btn-icon">✨</span>
              <span>Ask DocuPilot</span>
            </span>
          )}
        </button>
      </form>

      {(answer || isStreaming) && (
        <section className="answer-card" aria-live="polite">
          <div className="answer-header">
            <div className="answer-title-group">
              <span className="ai-gem-icon">💡</span>
              <h2>AI Grounded Answer</h2>
              {isStreaming && (
                <span className="streaming-badge">
                  <span className="pulse-dot" />
                  <span>Streaming live...</span>
                </span>
              )}
            </div>
            {answer && !isStreaming && (
              <button
                type="button"
                className={`btn-copy-answer ${copiedAnswer ? 'copied' : ''}`}
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
              <summary>
                <span>🔍 Retrieved Sources ({sources.length} passages)</span>
              </summary>
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
          <div className="history-card-header">
            <span className="history-icon">📜</span>
            <h2>Previous Questions & Answers</h2>
            <span className="history-count-badge">{history.length}</span>
          </div>
          <div className="history-list">
            {history.map((item) => (
              <div key={item.id} className="history-item">
                <div className="history-q-row">
                  <span className="history-q-badge">Q</span>
                  <p className="history-q">{item.question}</p>
                </div>
                <div className="history-a-row">
                  <span className="history-a-badge">A</span>
                  <div className="history-a">{item.answer}</div>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </>
  );
};

