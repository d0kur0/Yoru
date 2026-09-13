import { Children, isValidElement, type ReactNode } from 'react';
import { Select as Root, ListBox, Label } from '@heroui/react';
export function Select({children, value, defaultValue, onValueChange, 'aria-label': label}: {
  children: ReactNode; value?: string; defaultValue?: string; onValueChange?: (value: string) => void; 'aria-label': string;
}) {
  const options = Children.toArray(children).flatMap(child => {
    if (!isValidElement<{value?: string; children: ReactNode; disabled?: boolean}>(child)) return [];
    const text = String(child.props.children);
    return [{value: child.props.value ?? text, label: text, disabled: child.props.disabled}];
  });
  return <Root aria-label={label} value={value} defaultValue={defaultValue ?? options[0]?.value}
    onChange={key => { if (key !== null) onValueChange?.(String(key)); }} className="app-select">
    <Root.Trigger><Root.Value/><Root.Indicator/></Root.Trigger>
    <Root.Popover><ListBox aria-label={label}>
      {options.map(option => <ListBox.Item key={option.value} id={option.value} textValue={option.label} isDisabled={option.disabled}>
        <Label>{option.label}</Label><ListBox.ItemIndicator/>
      </ListBox.Item>)}
    </ListBox></Root.Popover>
  </Root>;
}
