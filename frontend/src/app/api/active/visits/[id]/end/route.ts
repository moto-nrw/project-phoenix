export async function POST(request: NextRequest, { params }: RouteParams) {
    try {
        const session = await auth();

        if (!session?.user?.token) {
            return NextResponse.json(
                { error: "Unauthorized" },
                { status: 401 }
            );
        }

        const response = await fetch(
            `${env.NEXT_PUBLIC_API_URL}/active/visits/${params.id}/end`,
            {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${session.user.token}`,
                    "Content-Type": "application/json",
                },
            }
        );

        if (!response.ok) {
            const errorText = await response.text();
            return NextResponse.json(
                { error: errorText },
                { status: response.status }
            );
        }

        const data = await response.json();
        return NextResponse.json(data);
    } catch (error) {
        console.error("End visit route error:", error);
        return NextResponse.json(
            { error: "Internal Server Error" },
            { status: 500 }
        );
    }
}