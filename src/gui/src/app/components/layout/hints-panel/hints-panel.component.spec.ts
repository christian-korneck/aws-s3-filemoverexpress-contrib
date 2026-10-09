import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { HINT_POPOVER_MODE, HintsPanelComponent } from './hints-panel.component';

describe('HintsPanelComponent', () => {
    let component: HintsPanelComponent;
    let fixture: ComponentFixture<HintsPanelComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                HintsPanelComponent,
            ],
            providers: [
                {
                    provide: HINT_POPOVER_MODE,
                    useValue: 'config',
                },
            ],
        })
            .compileComponents();

        fixture = TestBed.createComponent(HintsPanelComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });
});

describe('HintsPanelComponent S3-compatible provider hints', () => {
    async function render(mode: string): Promise<HTMLElement> {
        await TestBed.configureTestingModule({
            imports: [HintsPanelComponent],
            providers: [{provide: HINT_POPOVER_MODE, useValue: mode}],
        }).compileComponents();
        const fixture = TestBed.createComponent(HintsPanelComponent);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function headings(element: HTMLElement): string[] {
        return Array.from(element.querySelectorAll('h2')).map((heading) => heading.textContent?.trim() ?? '');
    }

    it('renders the storage provider help', async () => {
        const element = await render('transfer-profile-provider');
        expect(headings(element)).toContain('Storage provider');
        expect(headings(element)).toContain('Path-style addressing');
    });

    it('renders the TLS help', async () => {
        const element = await render('transfer-profile-tls');
        expect(headings(element)).toContain('Skip TLS certificate verification');
        expect(element.textContent).toContain('once per Remote');
    });

    it('renders the access keys help', async () => {
        const element = await render('transfer-profile-static-keys');
        expect(headings(element)).toEqual(['Access keys']);
    });
});
