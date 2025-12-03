#!/usr/bin/env python3
"""
Clean up kubebuilder and other annotations in generated swagger markdown.

Converts annotations to human-readable text in a "Validation" section.
"""

import re
import sys


def parse_annotations(text):
    """Extract annotations from text and return (clean_text, annotations_dict)."""
    annotations = {
        'optional': False,
        'required': False,
        'default': None,
        'enum_values': None,
        'minimum': None,
        'maximum': None,
        'min_length': None,
        'max_length': None,
        'min_items': None,
        'max_items': None,
        'pattern': None,
    }

    # Patterns to extract and their handlers
    # Order matters - more specific patterns first

    # +optional
    if re.search(r'\+optional\b', text):
        annotations['optional'] = True
    text = re.sub(r'</br>\+optional\b', '', text)
    text = re.sub(r'\+optional\b', '', text)

    # +required
    if re.search(r'\+required\b', text):
        annotations['required'] = True
    text = re.sub(r'</br>\+required\b', '', text)
    text = re.sub(r'\+required\b', '', text)

    # +default="value" or +default=value or +kubebuilder:default="value"
    match = re.search(r'\+(?:kubebuilder:)?default="([^"]*)"', text)
    if match:
        annotations['default'] = f'"{match.group(1)}"'
    else:
        match = re.search(r'\+(?:kubebuilder:)?default=(\S+?)(?:</br>|\s|\||$)', text)
        if match:
            annotations['default'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:default="[^"]*"', '', text)
    text = re.sub(r'</br>\+kubebuilder:default=\S+?(?=</br>|\s|\||$)', '', text)
    text = re.sub(r'\+kubebuilder:default="[^"]*"', '', text)
    text = re.sub(r'\+kubebuilder:default=\S+?(?=</br>|\s|\||$)', '', text)
    text = re.sub(r'</br>\+default="[^"]*"', '', text)
    text = re.sub(r'</br>\+default=\S+?(?=</br>|\s|\||$)', '', text)
    text = re.sub(r'\+default="[^"]*"', '', text)
    text = re.sub(r'\+default=\S+?(?=</br>|\s|\||$)', '', text)

    # +kubebuilder:validation:Enum=A;B;C
    match = re.search(r'\+kubebuilder:validation:Enum=([^|<\s]+)', text)
    if match:
        annotations['enum_values'] = match.group(1).replace(';', ', ')
    text = re.sub(r'</br>\+kubebuilder:validation:Enum=[^|<\s]+', '', text)
    text = re.sub(r'\+kubebuilder:validation:Enum=[^|<\s]+', '', text)

    # +kubebuilder:validation:Minimum=X
    match = re.search(r'\+kubebuilder:validation:Minimum=(\d+)', text)
    if match:
        annotations['minimum'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:Minimum=\d+', '', text)
    text = re.sub(r'\+kubebuilder:validation:Minimum=\d+', '', text)

    # +kubebuilder:validation:Maximum=X
    match = re.search(r'\+kubebuilder:validation:Maximum=(\d+)', text)
    if match:
        annotations['maximum'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:Maximum=\d+', '', text)
    text = re.sub(r'\+kubebuilder:validation:Maximum=\d+', '', text)

    # +kubebuilder:validation:MinLength=X
    match = re.search(r'\+kubebuilder:validation:MinLength=(\d+)', text)
    if match:
        annotations['min_length'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:MinLength=\d+', '', text)
    text = re.sub(r'\+kubebuilder:validation:MinLength=\d+', '', text)

    # +kubebuilder:validation:MaxLength=X
    match = re.search(r'\+kubebuilder:validation:MaxLength=(\d+)', text)
    if match:
        annotations['max_length'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:MaxLength=\d+', '', text)
    text = re.sub(r'\+kubebuilder:validation:MaxLength=\d+', '', text)

    # +kubebuilder:validation:MinItems=X
    match = re.search(r'\+kubebuilder:validation:MinItems=(\d+)', text)
    if match:
        annotations['min_items'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:MinItems=\d+', '', text)
    text = re.sub(r'\+kubebuilder:validation:MinItems=\d+', '', text)

    # +kubebuilder:validation:MaxItems=X
    match = re.search(r'\+kubebuilder:validation:MaxItems=(\d+)', text)
    if match:
        annotations['max_items'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:MaxItems=\d+', '', text)
    text = re.sub(r'\+kubebuilder:validation:MaxItems=\d+', '', text)

    # +kubebuilder:validation:Pattern=`X`
    match = re.search(r'\+kubebuilder:validation:Pattern=`([^`]+)`', text)
    if match:
        annotations['pattern'] = match.group(1)
    text = re.sub(r'</br>\+kubebuilder:validation:Pattern=`[^`]+`', '', text)
    text = re.sub(r'\+kubebuilder:validation:Pattern=`[^`]+`', '', text)

    # Handle +enum - just remove it (it's a type marker, not useful in docs)
    text = re.sub(r'\+enum\b', '', text)

    # Remove annotations we don't convert (internal implementation details)
    patterns_to_remove = [
        r'</br>\+kubebuilder:validation:XValidation:[^|<]*',
        r'\+kubebuilder:validation:XValidation:[^|]*',
        r'</br>\+kubebuilder:validation:Type=[^|<\s]+',
        r'\+kubebuilder:validation:Type=[^|\s]+',
        r'</br>\+kubebuilder:validation:Schemaless',
        r'\+kubebuilder:validation:Schemaless',
        r'</br>\+kubebuilder:pruning:PreserveUnknownFields',
        r'\+kubebuilder:pruning:PreserveUnknownFields',
        r'</br>\+kubebuilder:[^|<]*',
        r'\+kubebuilder:[^|]*',
        r'</br>\+patchStrategy=[^|<\s]+',
        r'\+patchStrategy=[^|\s]+',
        r'</br>\+patchMergeKey=[^|<\s]+',
        r'\+patchMergeKey=[^|\s]+',
        r'</br>\+listType=[^|<\s]+',
        r'\+listType=[^|\s]+',
        r'</br>\+listMapKey=[^|<\s]+',
        r'\+listMapKey=[^|\s]+',
        r'</br>\+featureGate=[^|<\s]+',
        r'\+featureGate=[^|\s]+',
        r'</br>\+structType=[^|<\s]+',
        r'\+structType=[^|\s]+',
        r'</br>\+protobuf[^|<]*',
        r'\+protobuf[^|]*',
        r'</br>\+union\b',
        r'\+union\b',
        r'</br>\+k8s:[^|<\s]+',
        r'\+k8s:[^|\s]+',
    ]

    for pattern in patterns_to_remove:
        text = re.sub(pattern, '', text)

    return text, annotations


def build_validation_section(annotations):
    """Build a human-readable validation section from annotations."""
    parts = []

    if annotations['optional']:
        parts.append('Optional')
    if annotations['required']:
        parts.append('Required')
    if annotations['default'] is not None:
        parts.append(f"Default: {annotations['default']}")
    if annotations['enum_values']:
        parts.append(f"Valid values: {annotations['enum_values']}")
    if annotations['minimum'] is not None:
        parts.append(f"Minimum: {annotations['minimum']}")
    if annotations['maximum'] is not None:
        parts.append(f"Maximum: {annotations['maximum']}")
    if annotations['min_length'] is not None:
        parts.append(f"Minimum length: {annotations['min_length']}")
    if annotations['max_length'] is not None:
        parts.append(f"Maximum length: {annotations['max_length']}")
    if annotations['min_items'] is not None:
        parts.append(f"Minimum items: {annotations['min_items']}")
    if annotations['max_items'] is not None:
        parts.append(f"Maximum items: {annotations['max_items']}")
    if annotations['pattern']:
        parts.append(f"Pattern: `{annotations['pattern']}`")

    if not parts:
        return ''

    return '</br>**Validation:** ' + '; '.join(parts) + '.'


def process_table_cell(cell):
    """Process a table cell that might contain annotations."""
    clean_text, annotations = parse_annotations(cell)
    validation_section = build_validation_section(annotations)

    # Clean up any trailing </br> before adding validation (but preserve cell spacing)
    clean_text = re.sub(r'(</br>)+\s*$', '', clean_text)

    if validation_section:
        clean_text = clean_text + validation_section

    return clean_text


def process_line(line):
    """Process a single line, handling table rows specially."""
    # Check if this is a table row
    if line.startswith('|') and '|' in line[1:]:
        # Split into cells
        cells = line.split('|')
        # Process each cell (skip first and last which are empty due to leading/trailing |)
        processed_cells = []
        for i, cell in enumerate(cells):
            if i == 0 or i == len(cells) - 1:
                processed_cells.append(cell)
            else:
                processed_cells.append(process_table_cell(cell))
        return '|'.join(processed_cells)

    # Check for standalone annotation lines like "> +kubebuilder:..." or "+kubebuilder:..."
    if re.match(r'^>?\s*\+kubebuilder:', line):
        return None  # Remove this line
    if re.match(r'^>?\s*\+enum\s*$', line):
        return None  # Remove standalone +enum lines
    if re.match(r'^>?\s*\+union\s*$', line):
        return None  # Remove standalone +union lines
    if re.match(r'^>?\s*\+structType=', line):
        return None
    if re.match(r'^>?\s*\+protobuf', line):
        return None

    # For description lines, also clean up annotations
    if '|' not in line:
        clean_text, annotations = parse_annotations(line)
        validation_section = build_validation_section(annotations)
        if validation_section:
            clean_text = clean_text.rstrip() + ' ' + validation_section.replace('</br>', ' ')
        # Clean up +enum from inline text
        clean_text = re.sub(r'\s*\+enum\s*', '', clean_text)
        return clean_text

    return line


def process_markdown(content):
    """Process the entire markdown content."""
    lines = content.split('\n')
    processed_lines = []

    for line in lines:
        processed = process_line(line)
        if processed is not None:
            processed_lines.append(processed)

    result = '\n'.join(processed_lines)

    # Also fix the interface{} links issue
    result = re.sub(r'\[interface{}\]\(#interface\)', '`interface{}`', result)

    # Clean up any multiple </br> (but don't touch other whitespace)
    result = re.sub(r'(</br>){2,}', '</br>', result)

    return result


def main():
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <markdown_file>", file=sys.stderr)
        sys.exit(1)

    filepath = sys.argv[1]

    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    processed = process_markdown(content)

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(processed)

    print(f"Processed {filepath}")


if __name__ == '__main__':
    main()
