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
    <div className="form-group">
      <label htmlFor="document-text">Raw Document Text / Notes:</label>
      <textarea
        id="document-text"
        rows={8}
        value={text}
        onChange={(e) => onChangeText(e.target.value)}
        placeholder="Paste raw document text or notes here..."
        disabled={disabled}
      />
    </div>
  );
};
