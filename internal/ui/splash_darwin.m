//go:build darwin

#import <Cocoa/Cocoa.h>

extern void goOnMacSplashClosed(void);
extern void goOnMacSplashCopy(const char* url);

@interface CFSplashCardView : NSView
@property (copy, nonatomic) NSString *authURL;
@property (strong, nonatomic) NSImage *logoImage;
@property (assign, nonatomic) int animTick;
@property (assign, nonatomic) BOOL copiedState;
@property (assign, nonatomic) BOOL successState;
@property (copy, nonatomic) NSString *statusText;
@property (strong, nonatomic) NSTimer *timer;
@end

@implementation CFSplashCardView

- (BOOL)isFlipped { return YES; }

- (instancetype)initWithFrame:(NSRect)frame authURL:(NSString *)url logo:(NSImage *)logo {
    self = [super initWithFrame:frame];
    if (self) {
        _authURL = [url copy];
        _logoImage = logo;
        _animTick = 0;
        _copiedState = NO;
        _successState = NO;
        _statusText = @"Esperando identificación...";
        _timer = [NSTimer scheduledTimerWithTimeInterval:0.033 target:self selector:@selector(onTick:) userInfo:nil repeats:YES];
    }
    return self;
}

- (void)onTick:(NSTimer *)t {
    self.animTick++;
    if (!self.successState) {
        int dots = (self.animTick / 10) % 4;
        NSMutableString *dStr = [NSMutableString stringWithString:@"Esperando identificación"];
        for (int i = 0; i < dots; i++) [dStr appendString:@"."];
        self.statusText = dStr;
    }
    [self setNeedsDisplay:YES];
}

- (void)drawRect:(NSRect)dirtyRect {
    CGFloat w = 380.0, h = 410.0;

    // 1. Fondo de tarjeta redondeada (#0B0F19 con borde #334155)
    NSBezierPath *bg = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(1, 1, w - 2, h - 2) xRadius:24 yRadius:24];
    [[NSColor colorWithCalibratedRed:11/255.0 green:15/255.0 blue:25/255.0 alpha:1.0] setFill];
    [bg fill];
    [[NSColor colorWithCalibratedRed:51/255.0 green:65/255.0 blue:85/255.0 alpha:1.0] setStroke];
    [bg setLineWidth:2.0];
    [bg stroke];

    // 2. Anillos concéntricos animados de Cloudflare
    CGFloat cx = w / 2.0, cy = 76.0;
    for (int i = 0; i < 3; i++) {
        int phase = (self.animTick + i * 12) % 36;
        CGFloat radius = 28.0 + (phase * 38.0 / 36.0);
        CGFloat alpha = (phase > 26) ? 0.25 : 0.85;
        [[NSColor colorWithCalibratedRed:244/255.0 green:129/255.0 blue:32/255.0 alpha:alpha] setStroke];
        NSBezierPath *ring = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(cx - radius, cy - radius, radius * 2, radius * 2)];
        [ring setLineWidth:2.0];
        [ring stroke];
    }

    // Core central con icono de Cloudflare
    NSBezierPath *core = [NSBezierPath bezierPathWithOvalInRect:NSMakeRect(cx - 34, cy - 34, 68, 68)];
    [[NSColor colorWithCalibratedRed:41/255.0 green:30/255.0 blue:20/255.0 alpha:1.0] setFill];
    [core fill];
    [[NSColor colorWithCalibratedRed:244/255.0 green:129/255.0 blue:32/255.0 alpha:1.0] setStroke];
    [core setLineWidth:1.0];
    [core stroke];

    if (self.logoImage) {
        [self.logoImage drawInRect:NSMakeRect(cx - 28, cy - 28, 56, 56) fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1.0];
    }

    // 3. Textos tipográficos
    NSMutableParagraphStyle *centerStyle = [[NSMutableParagraphStyle alloc] init];
    centerStyle.alignment = NSTextAlignmentCenter;

    NSDictionary *titleAttr = @{
        NSFontAttributeName: [NSFont systemFontOfSize:17 weight:NSFontWeightBold],
        NSForegroundColorAttributeName: [NSColor whiteColor],
        NSParagraphStyleAttributeName: centerStyle
    };
    [@"Vincular Cuenta Cloudflare" drawInRect:NSMakeRect(16, 138, w - 32, 24) withAttributes:titleAttr];

    NSDictionary *hintAttr = @{
        NSFontAttributeName: [NSFont systemFontOfSize:12 weight:NSFontWeightRegular],
        NSForegroundColorAttributeName: [NSColor colorWithCalibratedRed:168/255.0 green:184/255.0 blue:200/255.0 alpha:1.0],
        NSParagraphStyleAttributeName: centerStyle
    };
    [@"1. Abre o pega el enlace en tu navegador" drawInRect:NSMakeRect(16, 164, w - 32, 18) withAttributes:hintAttr];
    [@"2. Selecciona el dominio y pulsa «Authorize»" drawInRect:NSMakeRect(16, 180, w - 32, 18) withAttributes:hintAttr];

    NSColor *statColor = self.successState ? [NSColor colorWithCalibratedRed:52/255.0 green:211/255.0 blue:153/255.0 alpha:1.0] : [NSColor colorWithCalibratedRed:253/255.0 green:186/255.0 blue:116/255.0 alpha:1.0];
    NSDictionary *statAttr = @{
        NSFontAttributeName: [NSFont systemFontOfSize:13 weight:NSFontWeightSemibold],
        NSForegroundColorAttributeName: statColor,
        NSParagraphStyleAttributeName: centerStyle
    };
    [self.statusText drawInRect:NSMakeRect(16, 198, w - 32, 22) withAttributes:statAttr];

    // 4. Botón 1: Copiar Enlace
    NSColor *b1Col = self.copiedState ? [NSColor colorWithCalibratedRed:16/255.0 green:185/255.0 blue:129/255.0 alpha:1.0] : [NSColor colorWithCalibratedRed:244/255.0 green:129/255.0 blue:32/255.0 alpha:1.0];
    NSString *b1Txt = self.copiedState ? @"✅  ¡Enlace Copiado! Pégalo en tu navegador" : @"📋  Copiar Enlace de Identificación";
    NSBezierPath *btn1 = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(32, 224, w - 64, 44) xRadius:14 yRadius:14];
    [b1Col setFill];
    [btn1 fill];
    NSDictionary *b1Attr = @{
        NSFontAttributeName: [NSFont systemFontOfSize:13 weight:NSFontWeightBold],
        NSForegroundColorAttributeName: [NSColor colorWithCalibratedRed:11/255.0 green:15/255.0 blue:25/255.0 alpha:1.0],
        NSParagraphStyleAttributeName: centerStyle
    };
    [b1Txt drawInRect:NSMakeRect(36, 236, w - 72, 22) withAttributes:b1Attr];

    // 5. Botón 2: Abrir en Navegador
    NSBezierPath *btn2 = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(32, 280, w - 64, 42) xRadius:14 yRadius:14];
    [[NSColor colorWithCalibratedRed:34/255.0 green:51/255.0 blue:71/255.0 alpha:1.0] setFill];
    [btn2 fill];
    [[NSColor colorWithCalibratedRed:61/255.0 green:90/255.0 blue:122/255.0 alpha:1.0] setStroke];
    [btn2 setLineWidth:1.0];
    [btn2 stroke];
    NSDictionary *b2Attr = @{
        NSFontAttributeName: [NSFont systemFontOfSize:13 weight:NSFontWeightBold],
        NSForegroundColorAttributeName: [NSColor whiteColor],
        NSParagraphStyleAttributeName: centerStyle
    };
    [@"🌐  Abrir en Navegador" drawInRect:NSMakeRect(36, 291, w - 72, 22) withAttributes:b2Attr];

    // 6. Botón 3: Cancelar
    NSBezierPath *btn3 = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(125, 340, w - 250, 34) xRadius:12 yRadius:12];
    [[NSColor colorWithCalibratedRed:22/255.0 green:30/255.0 blue:41/255.0 alpha:1.0] setFill];
    [btn3 fill];
    [[NSColor colorWithCalibratedRed:51/255.0 green:65/255.0 blue:85/255.0 alpha:1.0] setStroke];
    [btn3 setLineWidth:1.0];
    [btn3 stroke];
    NSDictionary *b3Attr = @{
        NSFontAttributeName: [NSFont systemFontOfSize:12 weight:NSFontWeightMedium],
        NSForegroundColorAttributeName: [NSColor colorWithCalibratedRed:148/255.0 green:163/255.0 blue:184/255.0 alpha:1.0],
        NSParagraphStyleAttributeName: centerStyle
    };
    [@"Cancelar" drawInRect:NSMakeRect(125, 348, w - 250, 20) withAttributes:b3Attr];
}

- (void)mouseDown:(NSEvent *)event {
    NSPoint p = [self convertPoint:event.locationInWindow fromView:nil];

    // Botón 1: Copiar Enlace
    if (p.x >= 32 && p.x <= 348 && p.y >= 224 && p.y <= 268) {
        goOnMacSplashCopy([self.authURL UTF8String]);
        self.copiedState = YES;
        [self setNeedsDisplay:YES];
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(3.0 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
            self.copiedState = NO;
            [self setNeedsDisplay:YES];
        });
        return;
    }

    // Botón 2: Abrir en Navegador
    if (p.x >= 32 && p.x <= 348 && p.y >= 280 && p.y <= 322) {
        [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:self.authURL]];
        return;
    }

    // Botón 3: Cancelar
    if (p.x >= 125 && p.x <= 255 && p.y >= 340 && p.y <= 374) {
        [self.window close];
        return;
    }

    // Arrastrar ventana desde cualquier área libre
    [self.window performWindowDragWithEvent:event];
}

- (void)dealloc {
    [self.timer invalidate];
}

@end

static NSWindow *g_splashWindow = nil;
static CFSplashCardView *g_splashCard = nil;

void showMacSplash(const char* authURL, const void* pngBytes, int pngLen) {
    @autoreleasepool {
        NSString *urlStr = [NSString stringWithUTF8String:authURL];
        NSImage *logo = nil;
        if (pngBytes != NULL && pngLen > 0) {
            NSData *d = [NSData dataWithBytes:pngBytes length:pngLen];
            logo = [[NSImage alloc] initWithData:d];
        }

        dispatch_async(dispatch_get_main_queue(), ^{
            if (g_splashWindow) {
                [g_splashWindow close];
                g_splashWindow = nil;
            }

            NSRect r = NSMakeRect(0, 0, 380, 410);
            g_splashWindow = [[NSWindow alloc] initWithContentRect:r
                                                         styleMask:(NSWindowStyleMaskBorderless)
                                                           backing:NSBackingStoreBuffered
                                                             defer:NO];
            [g_splashWindow setOpaque:NO];
            [g_splashWindow setBackgroundColor:[NSColor clearColor]];
            [g_splashWindow setLevel:NSFloatingWindowLevel];
            [g_splashWindow setHasShadow:YES];
            [g_splashWindow center];

            g_splashCard = [[CFSplashCardView alloc] initWithFrame:r authURL:urlStr logo:logo];
            [g_splashWindow setContentView:g_splashCard];

            [[NSNotificationCenter defaultCenter] addObserverForName:NSWindowWillCloseNotification
                                                              object:g_splashWindow
                                                               queue:[NSOperationQueue mainQueue]
                                                          usingBlock:^(NSNotification *note) {
                goOnMacSplashClosed();
                g_splashWindow = nil;
                g_splashCard = nil;
            }];

            [NSApp activateIgnoringOtherApps:YES];
            [g_splashWindow makeKeyAndOrderFront:nil];
        });
    }
}

void updateMacSplashSuccess(const char* accName) {
    @autoreleasepool {
        NSString *name = [NSString stringWithUTF8String:accName];
        dispatch_async(dispatch_get_main_queue(), ^{
            if (g_splashCard) {
                g_splashCard.successState = YES;
                g_splashCard.statusText = [NSString stringWithFormat:@"✅ ¡Cuenta %@ vinculada!", name];
                [g_splashCard setNeedsDisplay:YES];
                dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(2.0 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
                    [g_splashWindow close];
                });
            }
        });
    }
}

void closeMacSplash(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_splashWindow) {
            [g_splashWindow close];
        }
    });
}
