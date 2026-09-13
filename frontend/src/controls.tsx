import { Button as HeroButton } from '@heroui/react';
import type { ButtonHTMLAttributes, ComponentProps } from 'react';
export function Button({className = '', disabled, children, ...props}: ButtonHTMLAttributes<HTMLButtonElement>) {
  const primary = className.split(' ').includes('primary');
  const action = className.split(' ').includes('button');
  const icon = className.includes('icon-button');
  return <HeroButton {...props as ComponentProps<typeof HeroButton>} type={props.type ?? 'submit'}
    isDisabled={disabled} isIconOnly={icon} variant={primary ? 'primary' : action ? 'secondary' : 'ghost'}
    className={className.split(' ').filter(name => name !== 'button' && name !== 'primary').join(' ')}>{children}</HeroButton>;
}
