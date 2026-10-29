import React from 'react';

interface PasteTextTabProps {
  text: string;
  onChangeText: (text: string) => void;
  disabled: boolean;
}

export const PasteTextTab: React.FC<PasteTextTabProps> = ({
  text,
  onChangeText,
  disabled,
}) => {
  return (
    <div className="form-group paste-tab-group">
      <div className="label-row">
        <label htmlFor="document-text">Raw Document Text / Notes:</label>
        <div className="paste-meta-row">
          <span className="label-hint">
            {text.length > 0 ? `${text.length.toLocaleString()} characters` : 'Paste custom text'}
          </span>
          {text.length > 0 && !disabled && (
            <button
              type="button"
              className="btn-clear-text"
              onClick={() => onChangeText('')}
              title="Clear pasted text"
            >
              Clear
            </button>
          )}
        </div>
      </div>
      <textarea
        id="document-text"
        rows={8}
        value={text}
        onChange={(e) => onChangeText(e.target.value)}
        placeholder="Paste articles, research notes, transcripts, or arbitrary text here to ask questions about it..."
        disabled={disabled}
      />
    </div>
  );
};

