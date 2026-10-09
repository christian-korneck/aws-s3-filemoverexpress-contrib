import { NgTemplateOutlet } from '@angular/common';
import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { MatIcon } from '@angular/material/icon';
import { isPackagedApp } from '@app/utils/utils';
import { WailsService } from '@services/wails/wails.service';
import { HINT_POPOVER_MODE } from './hints-panel.token';

export { HINT_POPOVER_MODE } from './hints-panel.token';

@Component({
    selector: 'fme-hints-panel',
    templateUrl: './hints-panel.component.html',
    styleUrls: ['./hints-panel.component.scss'],
    changeDetection: ChangeDetectionStrategy.Eager,
    imports: [MatIcon, NgTemplateOutlet],
})
export class HintsPanelComponent {
    private wails = inject(WailsService);
    mode = inject(HINT_POPOVER_MODE);

    openExternalLink(event: Event, url: string) {
        event.preventDefault();

        if (isPackagedApp()) {
            this.wails.externalLink(url).subscribe();
        }
    }
}
