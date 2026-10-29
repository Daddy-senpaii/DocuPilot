import React, { useRef, useState } from 'react';
import { ConversionPair, ConversionResult, DEFAULT_CONVERSION_PAIRS } from '../types';

interface ConverterStudioTabProps {
  selectedPair: ConversionPair;
  onSelectPair: (pair: ConversionPair) => void;
  convFile: File | null;
  onFileSelect: (file: File) => void;
  convLoading: boolean;
  convError: string | null;
  convResult: ConversionResult | null;
  onRunConversion: () => void;
  onSendConvertedToQA: () => void;
  onReset: () => void;
}

export const ConverterStudioTab: React.FC<ConverterStudioTabProps> = ({
  selectedPair,
  onSelectPair,
  convFile,
  onFileSelect,
  convLoading,
  convError,
  convResult,
  onRunConversion,
  onSendConvertedToQA,
  onReset,
}) => {
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  return (
    <div className="converter-section">
      <div className="converter-header">
        <div className="converter-title-row">
          <h3>Fast & High-Fidelity Document Converter</h3>
          <span className="converter-engine-badge">⚡ 8 Formats Supported</span>
        </div>
        <p className="converter-subtitle">
          Convert seamlessly between PDF, Word (.docx), Excel (.xlsx), PowerPoint (.pptx), and Text (.txt).
        </p>
      </div>

      {/* Quick Conversion Workflows */}
      <div className="conversion-presets">
        <span className="presets-label">Choose Conversion:</span>
        <div className="presets-grid">
          {DEFAULT_CONVERSION_PAIRS.map((pair) => {
            const isSelected =
              selectedPair.source_format === pair.source_format &&
              selectedPair.target_format === pair.target_format;
            return (
              <button
                key={`${pair.source_format}->${pair.target_format}`}
                type="button"
                className={`preset-chip ${isSelected ? 'selected' : ''}`}
                onClick={() => onSelectPair(pair)}
              >
                <span className="preset-icon">{pair.icon}</span>
                <span className="preset-title">{pair.title}</span>
                {isSelected && <span className="preset-active-dot" />}
              </button>
            );
          })}
        </div>
      </div>

      {/* Drag & Drop Zone */}
      <div
        className={`upload-dropzone converter-dropzone ${dragOver ? 'drag-over' : ''} ${convFile ? 'has-file' : ''}`}
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragOver(false);
          const file = e.dataTransfer.files?.[0];
          if (file) onFileSelect(file);
        }}
        onClick={() => fileInputRef.current?.click()}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept=".pdf,.docx,.xlsx,.pptx,.txt"
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) onFileSelect(file);
          }}
          disabled={convLoading}
          style={{ display: 'none' }}
        />

        <div className="upload-icon">
          {convFile ? '📑' : '🔄'}
        </div>

        {convFile ? (
          <div className="converter-file-details">
            <p className="converter-selected-name">
              <strong>{convFile.name}</strong>
            </p>
            <span className="converter-file-size">
              {(convFile.size / 1024).toFixed(1)} KB • Source: .{selectedPair.source_format.toUpperCase()}
            </span>
            <p className="converter-change-hint">Click or drop another file to change</p>
          </div>
        ) : (
          <>
            <p className="upload-prompt">
              Click or drop your <strong>.{selectedPair.source_format.toUpperCase()}</strong> file here
            </p>
            <span className="converter-hint">
              Accepts .{selectedPair.source_format.toUpperCase()} (up to 10MB)
            </span>
          </>
        )}
      </div>

      {/* Target Format Selector & Convert Action */}
      <div className="converter-action-bar">
        <div className="format-flow-badge">
          <span className="flow-badge-part source">.{selectedPair.source_format.toUpperCase()}</span>
          <span className="flow-arrow">➡️</span>
          <span className="flow-badge-part target">.{selectedPair.target_format.toUpperCase()}</span>
        </div>

        <button
          type="button"
          className="btn-primary converter-submit-btn"
          onClick={onRunConversion}
          disabled={!convFile || convLoading}
        >
          {convLoading ? (
            <span className="btn-loading-content">
              <span className="spinner" />
              Converting Document...
            </span>
          ) : (
            `Convert to .${selectedPair.target_format.toUpperCase()}`
          )}
        </button>
      </div>

      {convError && (
        <div className="alert-error converter-alert" role="alert">
          <strong>Conversion Error:</strong> {convError}
          {convError.includes('LibreOffice') && (
            <div className="install-hint">
              <code>sudo apt update && sudo apt install -y libreoffice</code>
            </div>
          )}
        </div>
      )}

      {/* Converted Output Card */}
      {convResult && (
        <div className="converter-result-card" aria-live="polite">
          <div className="result-header">
            <span className="result-badge-check">✅</span>
            <div>
              <h4 className="result-title">Conversion Complete!</h4>
              <p className="result-meta">
                <strong>{convResult.filename}</strong> ({(convResult.size / 1024).toFixed(1)} KB)
              </p>
            </div>
          </div>

          <div className="result-actions">
            <a
              href={convResult.blobUrl}
              download={convResult.filename}
              className="btn-download"
            >
              📥 Download {convResult.filename}
            </a>

            {(convResult.targetFormat === 'pdf' || convResult.targetFormat === 'txt') && (
              <button
                type="button"
                className="btn-chat-doc"
                onClick={onSendConvertedToQA}
              >
                🤖 Ask Questions in DocuPilot
              </button>
            )}

            <button
              type="button"
              className="btn-secondary"
              onClick={onReset}
            >
              Convert Another
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
